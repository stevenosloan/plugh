package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBin makes a folder holding executables that print their name and
// arguments. A go that's given "tool -n" exits with toolStatus, which says
// whether the project has plugh as a Go tool.
func fakeBin(t *testing.T, toolStatus string, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		script := "#!/bin/sh\necho " + name + " \"$@\"\n"
		if name == "go" {
			script = "#!/bin/sh\n[ \"$1 $2\" = 'tool -n' ] && exit " + toolStatus + "\necho go \"$@\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// shim runs the plugin's bin/plugh in dir with only path on PATH, and
// returns what it printed to stdout and stderr. It gives up after a few
// seconds, so a shim that loops fails the test instead of hanging it.
func shim(t *testing.T, dir string, path []string, args ...string) (string, string, error) {
	t.Helper()
	bin, err := filepath.Abs("../../bin")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(bin, "plugh"), args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + strings.Join(append([]string{bin}, path...), ":")}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("shim %v timed out; is it looping?", args)
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// shimCopy puts a copy of bin/plugh in a folder of its own, like a second
// load of the plugin.
func shimCopy(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../bin/plugh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugh"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeRouter writes a voice.yaml into dir.
func writeRouter(t *testing.T, dir, router string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "voice.yaml"), []byte(router), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShimPrefersPlughOnPath(t *testing.T) {
	got, _, err := shim(t, t.TempDir(), []string{fakeBin(t, "1", "plugh", "go")}, "kit")
	if err != nil || got != "plugh kit" {
		t.Errorf("shim = %q, %v; want the plugh on PATH", got, err)
	}
}

func TestShimUsesGoTool(t *testing.T) {
	got, _, err := shim(t, t.TempDir(), []string{fakeBin(t, "0", "go")}, "check", "docs")
	if err != nil || got != "go tool plugh check docs" {
		t.Errorf("shim = %q, %v; want go tool", got, err)
	}
}

func TestShimPrefersGoToolToPlughOnPath(t *testing.T) {
	// A project that pins plugh in go.mod gets that one, not a global install.
	got, _, err := shim(t, t.TempDir(), []string{fakeBin(t, "0", "plugh", "go")}, "kit")
	if err != nil || got != "go tool plugh kit" {
		t.Errorf("shim = %q, %v; want go tool", got, err)
	}
}

func TestShimSkipsOtherShims(t *testing.T) {
	// Two loads of the plugin: each shim has to skip the other, not run it.
	real := fakeBin(t, "1", "plugh")
	got, _, err := shim(t, t.TempDir(), []string{shimCopy(t), real}, "kit")
	if err != nil || got != "plugh kit" {
		t.Errorf("shim = %q, %v; want the real plugh after both shims", got, err)
	}
	_, msg, err := shim(t, t.TempDir(), []string{shimCopy(t)}, "kit")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 127 || !strings.Contains(msg, "not installed") {
		t.Errorf("shim = %q, %v; want exit 127 saying it's not installed", msg, err)
	}
}

func TestShimSkipsEmptyPathEntry(t *testing.T) {
	// An empty entry means the current folder; a plugh there isn't run.
	dir := fakeBin(t, "1", "plugh")
	got, _, err := shim(t, dir, []string{"", fakeBin(t, "1", "go")}, "kit")
	if want := "go run github.com/stevenosloan/plugh/cmd/plugh@latest kit"; err != nil || got != want {
		t.Errorf("shim = %q, %v; want %q", got, err, want)
	}
}

func TestShimRunsPinnedVersion(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "docs", "guides")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRouter(t, root, "personas: {}\nplugh: \"v0.2.0\"  # the pin\ntypes: {}\n")
	gobin := fakeBin(t, "1", "go")

	got, msg, err := shim(t, sub, []string{gobin}, "kit")
	if want := "go run github.com/stevenosloan/plugh/cmd/plugh@v0.2.0 kit"; err != nil || got != want {
		t.Errorf("shim = %q, %v; want %q", got, err, want)
	}
	if msg != "" {
		t.Errorf("a pinned run printed %q to stderr", msg)
	}

	got, msg, err = shim(t, t.TempDir(), []string{gobin}, "kit") // no voice.yaml
	if want := "go run github.com/stevenosloan/plugh/cmd/plugh@latest kit"; err != nil || got != want {
		t.Errorf("shim = %q, %v; want %q", got, err, want)
	}
	if !strings.Contains(msg, "go install it, or pin a version") {
		t.Errorf("stderr = %q, want a hint to install or pin", msg)
	}
}

func TestShimReadsWindowsLineEndings(t *testing.T) {
	root := t.TempDir()
	writeRouter(t, root, "personas: {}\r\nplugh: v0.2.0\r\ntypes: {}\r\n")
	got, _, err := shim(t, root, []string{fakeBin(t, "1", "go")}, "kit")
	if want := "go run github.com/stevenosloan/plugh/cmd/plugh@v0.2.0 kit"; err != nil || got != want {
		t.Errorf("shim = %q, %v; want %q", got, err, want)
	}
}

func TestShimWithNothingInstalled(t *testing.T) {
	// Only the shim itself on PATH: it has to skip itself, not loop.
	_, msg, err := shim(t, t.TempDir(), nil, "kit")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 127 || !strings.Contains(msg, "not installed") {
		t.Errorf("shim = %q, %v; want exit 127 saying it's not installed", msg, err)
	}
}
