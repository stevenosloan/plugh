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
	if !strings.HasPrefix(out.String(), "ok: 4 types") {
		t.Errorf("kit output = %q", out.String())
	}
}
