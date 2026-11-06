package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// project makes a throwaway copy of the example project with one guide, and
// moves into it for the test.
func project(t *testing.T, guide string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../examples")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"docs/guides/install.md": guide,
		"readme.md":              "Simply not content, so not checked!",
	}
	for name, data := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join(dir, "docs")) // a subfolder, so findRoot has to walk up
	return dir
}

func TestCheck(t *testing.T) {
	project(t, "---\ntitle: x\n---\n\nSimply run it, e.g. now.  Done!\n")
	var out strings.Builder
	err := run([]string{"check"}, &out)
	if !errors.Is(err, errFindings) {
		t.Fatalf("run(check) = %v, want errFindings", err)
	}
	for _, want := range []string{
		"install.md:5:1 minimizers:",
		"install.md:5:16 latin-abbreviations:",
		"install.md:5:24 double-space:",
		"install.md:5:30 exclamation:",
		"2 errors, 2 warnings",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "readme.md") {
		t.Errorf("checked readme.md, which no type covers:\n%s", out.String())
	}
}

func TestContext(t *testing.T) {
	project(t, "Fine.\n")
	var out strings.Builder
	if err := run([]string{"context", "guides/install.md"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"# Voice for guide (docs/guides/)", "## Writing as maintainer", "## Writing for newcomer", "<!-- chunk voice-core,", "<!-- chunk guide,", "- code-samples: "} {
		if !strings.Contains(got, want) {
			t.Errorf("context is missing %q", want)
		}
	}
	if strings.Index(got, "chunk voice-core") > strings.Index(got, "chunk guide") {
		t.Error("voice-core should come before guide, which depends on it")
	}
}

func TestKit(t *testing.T) {
	project(t, "")
	var out strings.Builder
	if err := run([]string{"kit"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ok: 5 types") {
		t.Errorf("kit output = %q", out.String())
	}
}

func TestVersion(t *testing.T) {
	t.Chdir(t.TempDir()) // no voice.yaml anywhere it looks
	var out strings.Builder
	if err := run([]string{"version"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "plugh ") {
		t.Errorf("version output = %q", out.String())
	}
}

func TestKitWarnsOnPinMismatch(t *testing.T) {
	project(t, "") // examples/voice.yaml pins v0.3.0
	for v, warn := range map[string]bool{"v9.9.9": true, "v0.2.0": true, "v0.3.0": false, "v0.3.3": false, devel: false} {
		t.Run(v, func(t *testing.T) {
			old := version
			version = v
			t.Cleanup(func() { version = old })
			var out strings.Builder
			if err := run([]string{"kit"}, &out); err != nil {
				t.Fatal(err)
			}
			got := strings.Contains(out.String(), "warning: ")
			if got != warn {
				t.Errorf("version %s: warned = %v, want %v:\n%s", v, got, warn, out.String())
			}
			if !strings.Contains(out.String(), "ok: 5 types") {
				t.Errorf("kit output = %q", out.String())
			}
		})
	}
}

// withVersion runs the test as a build of plugh at v.
func withVersion(t *testing.T, v string) {
	old := version
	version = v
	t.Cleanup(func() { version = old })
}

func TestContextWarnsOnPinMismatch(t *testing.T) {
	project(t, "Fine.\n") // examples/voice.yaml pins v0.3.0
	withVersion(t, "v9.9.9")
	var out, errOut strings.Builder
	old := stderr
	stderr = &errOut
	t.Cleanup(func() { stderr = old })

	if err := run([]string{"context", "guide"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "plugh: warning: ") {
		t.Errorf("stderr = %q, want a warning", errOut.String())
	}
	if strings.Contains(out.String(), "warning") {
		t.Errorf("the warning got into the context:\n%s", out.String())
	}
}

func TestKitWarnsEvenWhenInvalid(t *testing.T) {
	dir := project(t, "")
	router := filepath.Join(dir, "voice.yaml")
	data, err := os.ReadFile(router)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, "  broken:\n    always: [no-such-chunk]\n"...)
	if err := os.WriteFile(router, data, 0o644); err != nil {
		t.Fatal(err)
	}
	withVersion(t, "v9.9.9")
	var out strings.Builder
	if err := run([]string{"kit"}, &out); err == nil {
		t.Fatal("kit passed a router with a missing chunk")
	}
	if !strings.Contains(out.String(), "warning: ") {
		t.Errorf("kit output = %q, want the warning ahead of the errors", out.String())
	}
}

func TestCheckRoutesByFrontMatter(t *testing.T) {
	// No path covers docs/v1.md, so only its front matter gets it checked.
	dir := project(t, "")
	if err := os.WriteFile(filepath.Join(dir, "docs", "v1.md"), []byte("---\nkind: release-note\n---\n\nSimply works now.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "bad.md"), []byte("---\nkind: [x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err := run([]string{"check"}, &out)
	if !errors.Is(err, errFindings) {
		t.Fatalf("run(check) = %v, want errFindings", err)
	}
	for _, want := range []string{"v1.md:5:1 minimizers:", "bad.md: front matter:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
}

func TestContextUsesFrontMatter(t *testing.T) {
	project(t, "---\nkind: troubleshooting\n---\n\nFine.\n")
	var out strings.Builder
	if err := run([]string{"context", "guides/install.md"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := "# Voice for troubleshooting (docs/guides/ with kind: troubleshooting)"; !strings.HasPrefix(out.String(), want) {
		t.Errorf("context starts %q, want %q", strings.SplitN(out.String(), "\n", 2)[0], want)
	}
	if !strings.Contains(out.String(), "## Writing for regular") {
		t.Error("context should use troubleshooting's audience")
	}
}

func TestContextSuggestsForNewFile(t *testing.T) {
	project(t, "")
	var out strings.Builder
	if err := run([]string{"context", "guides/new.md"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"# Voice for guide (docs/guides/)",
		"doesn't exist yet",
		"- docs/guides/ with kind: troubleshooting: run `plugh context troubleshooting` instead",
		"- front matter kind: release-note: run `plugh context release-note` instead",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("context is missing %q:\n%s", want, got)
		}
	}

	// A job asked for by name says what it's for, not its match.
	out.Reset()
	if err := run([]string{"context", "release-note"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := "# Voice for release-note (the summary at the top of a GitHub release)"; !strings.HasPrefix(out.String(), want) {
		t.Errorf("context starts %q, want %q", strings.SplitN(out.String(), "\n", 2)[0], want)
	}
}

func TestContextNewFileOnlyFrontMatterFits(t *testing.T) {
	project(t, "")
	var out strings.Builder
	err := run([]string{"context", "notes/new.md"}, &out)
	if err == nil || !strings.Contains(err.Error(), "matches no type by its path alone") || !strings.Contains(err.Error(), "plugh context release-note") {
		t.Errorf("context = %v, want an error suggesting release-note", err)
	}
}
