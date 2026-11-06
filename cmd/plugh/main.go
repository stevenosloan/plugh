// Command plugh prints the voice context for a file and checks markdown
// against it. Run it from anywhere inside a project with a voice.yaml.
//
//	plugh context <file or type>   what an agent should read before writing
//	plugh check [path ...]         run the checks for each markdown file's type
//	plugh chunk <id>               print one chunk, like an if_relevant one
//	plugh kit                      check the router and chunks themselves
//	plugh version                  print this build's version
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/stevenosloan/plugh/internal/voice"
)

const usage = `usage:
  plugh context <file or type>   print what an agent should read before writing
  plugh check [path ...]         check markdown files (default: the whole project)
  plugh chunk <id>               print one chunk, like an if_relevant one
  plugh kit                      check the router and chunks themselves
  plugh version                  print this build's version
`

// stderr is where warnings and usage go. A var so tests can read it.
var stderr io.Writer = os.Stderr

// errFindings means a check found errors; they've already been printed.
var errFindings = errors.New("found errors")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if !errors.Is(err, errFindings) {
			fmt.Fprintln(os.Stderr, "plugh:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("no command given")
	}
	cmd, args := args[0], args[1:]
	if cmd == "version" { // needs no project
		fmt.Fprintf(out, "plugh %s\n", version)
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := findRoot(wd)
	if err != nil {
		return err
	}
	kit, loadErr := voice.Load(os.DirFS(root))
	// kit prints this itself. The rest warn on stderr, so it can't get mixed
	// into the context or findings they print.
	if kit != nil && cmd != "kit" {
		if w := pinWarning(kit.Plugh, version); w != "" {
			fmt.Fprintf(stderr, "plugh: warning: %s\n", w)
		}
	}

	switch cmd {
	case "kit":
		return kitCmd(kit, loadErr, out)
	case "context":
		if loadErr != nil {
			return loadErr
		}
		if len(args) != 1 {
			return errors.New("context takes one file or type")
		}
		return contextCmd(kit, root, wd, args[0], out)
	case "chunk":
		if loadErr != nil {
			return loadErr
		}
		if len(args) != 1 {
			return errors.New("chunk takes one chunk id")
		}
		return chunkCmd(kit, args[0], out)
	case "check":
		if loadErr != nil {
			return loadErr
		}
		if len(args) == 0 {
			args = []string{root}
		}
		return checkCmd(kit, root, args, out)
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

// findRoot walks up from dir to the nearest folder holding a voice.yaml.
func findRoot(dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, voice.RouterFile)); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("no %s in %s or any folder above it", voice.RouterFile, dir)
		}
	}
}

// relToRoot turns a path given on the command line into the slash-separated,
// root-relative form a type's match is written in.
func relToRoot(root, wd, p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(wd, p)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
