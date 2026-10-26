package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stevenosloan/plugh/internal/voice"
)

func kitCmd(kit *voice.Kit, loadErr error, out io.Writer) error {
	if kit == nil {
		return loadErr
	}
	if err := errors.Join(loadErr, kit.Validate()); err != nil {
		return err
	}
	fmt.Fprintf(out, "ok: %d types, %d chunks\n", len(kit.Types), len(kit.Chunks))
	return nil
}

func contextCmd(kit *voice.Kit, root, wd, arg string, out io.Writer) error {
	t, ok := kit.Types[arg]
	if !ok {
		rel, err := relToRoot(root, wd, arg)
		if err != nil {
			return err
		}
		if t, ok = kit.TypeFor(rel); !ok {
			return fmt.Errorf("%s matches no type in %s; types are %s", arg, voice.RouterFile,
				strings.Join(slices.Sorted(maps.Keys(kit.Types)), ", "))
		}
	}
	always, relevant, err := kit.Resolve(t)
	if err != nil {
		return err
	}
	role, audience, err := kit.PersonasFor(t)
	if err != nil {
		return err
	}

	what := t.Match
	if what == "" {
		what = t.Doc
	}
	fmt.Fprintf(out, "# Voice for %s (%s)\n\n", t.Name, what)
	fmt.Fprintf(out, "Follow these chunks. Where they're silent, follow their \"When unsure\" lines.\n")
	if role != nil {
		fmt.Fprintf(out, "\n## Writing as %s\n\n%s\n", role.Name, strings.TrimSpace(role.Description))
	}
	if audience != nil {
		fmt.Fprintf(out, "\n## Writing for %s\n\n%s\n", audience.Name, strings.TrimSpace(audience.Description))
	}
	for _, c := range always {
		fmt.Fprintf(out, "\n<!-- chunk %s, from %s -->\n%s", c.ID, c.Source, strings.TrimLeft(c.Body, "\n"))
	}
	if len(relevant) > 0 {
		fmt.Fprintf(out, "\n## Load if relevant\n\nIf a summary applies to what you're writing, run `go tool plugh chunk <id>` and follow it too.\n\n")
		for _, c := range relevant {
			fmt.Fprintf(out, "- %s: %s\n", c.ID, c.Summary)
		}
	}
	return nil
}

func chunkCmd(kit *voice.Kit, id string, out io.Writer) error {
	c, ok := kit.Chunks[id]
	if !ok {
		return fmt.Errorf("no chunk %q", id)
	}
	fmt.Fprintf(out, "<!-- chunk %s, from %s -->\n%s", c.ID, c.Source, strings.TrimLeft(c.Body, "\n"))
	return nil
}

func checkCmd(kit *voice.Kit, root string, paths []string, out io.Writer) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	var files []string
	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && path != p && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir // .git and friends
			}
			if !d.IsDir() && filepath.Ext(path) == ".md" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	var errs, warns int
	for _, file := range files {
		rel, err := relToRoot(root, wd, file)
		if err != nil {
			return err
		}
		t, ok := kit.TypeFor(rel)
		if !ok {
			continue // not content any type covers, like a readme
		}
		checks, err := kit.Checks(t)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, f := range voice.CheckMarkdown(src, checks) {
			label := ""
			if f.Check.Severity == voice.Warn {
				label = " (warning)"
				warns++
			} else {
				errs++
			}
			fmt.Fprintf(out, "%s:%s%s\n", file, f, label)
		}
	}
	fmt.Fprintf(out, "%d %s, %d %s\n", errs, plural(errs, "error"), warns, plural(warns, "warning"))
	if errs > 0 {
		return errFindings
	}
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
