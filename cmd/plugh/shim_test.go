package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

// shim runs the plugin's bin/plugh in dir with only path on PATH.
func shim(t *testing.T, dir string, path []string, args ...string) (string, error) {
	t.Helper()
	bin, err := filepath.Abs("../../bin")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(bin, "plugh"), args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + strings.Join(append([]string{bin}, path...), ":")}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func TestShimPrefersPlughOnPath(t *testing.T) {
	got, err := shim(t, t.TempDir(), []string{fakeBin(t, "1", "plugh", "go")}, "kit")
	if err != nil || got != "plugh kit" {
		t.Errorf("shim = %q, %v; want the plugh on PATH", got, err)
	}
}

func TestShimUsesGoTool(t *testing.T) {
	got, err := shim(t, t.TempDir(), []string{fakeBin(t, "0", "go")}, "check", "docs")
	if err != nil || got != "go tool plugh check docs" {
		t.Errorf("shim = %q, %v; want go tool", got, err)
	}
}

func TestShimRunsPinnedVersion(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "docs", "guides")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	router := "personas: {}\nplugh: \"v0.2.0\"  # the pin\ntypes: {}\n"
	if err := os.WriteFile(filepath.Join(root, "voice.yaml"), []byte(router), 0o644); err != nil {
		t.Fatal(err)
	}
	gobin := fakeBin(t, "1", "go")

	got, err := shim(t, sub, []string{gobin}, "kit")
	if want := "go run github.com/stevenosloan/plugh/cmd/plugh@v0.2.0 kit"; err != nil || got != want {
		t.Errorf("shim = %q, %v; want %q", got, err, want)
	}
	got, err = shim(t, t.TempDir(), []string{gobin}, "kit") // no voice.yaml
	if want := "go run github.com/stevenosloan/plugh/cmd/plugh@latest kit"; err != nil || got != want {
		t.Errorf("shim = %q, %v; want %q", got, err, want)
	}
}

func TestShimWithNothingInstalled(t *testing.T) {
	// Only the shim itself on PATH: it has to skip itself, not loop.
	got, err := shim(t, t.TempDir(), nil, "kit")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 127 || !strings.Contains(got, "not installed") {
		t.Errorf("shim = %q, %v; want exit 127 saying it's not installed", got, err)
	}
}
