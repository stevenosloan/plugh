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
	// Before validating: a version mismatch can be why it doesn't validate.
	if w := pinWarning(kit.Plugh, version); w != "" {
		fmt.Fprintf(out, "warning: %s\n", w)
	}
	if err := errors.Join(loadErr, kit.Validate()); err != nil {
		return err
	}
	fmt.Fprintf(out, "ok: %d types, %d chunks\n", len(kit.Types), len(kit.Chunks))
	return nil
}

func contextCmd(kit *voice.Kit, root, wd, arg string, out io.Writer) error {
	t, what, suggest, err := typeForContext(kit, root, wd, arg)
	if err != nil {
		return err
	}
	always, relevant, err := kit.Resolve(t)
	if err != nil {
		return err
	}
	role, audience, err := kit.PersonasFor(t)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "# Voice for %s (%s)\n\n", t.Name, what)
	if suggest != "" {
		fmt.Fprintf(out, "%s\n\n", suggest)
	}
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
		fmt.Fprintf(out, "\n## Load if relevant\n\nIf a summary applies to what you're writing, run `plugh chunk <id>` and follow it too.\n\n")
		for _, c := range relevant {
			fmt.Fprintf(out, "- %s: %s\n", c.ID, c.Summary)
		}
	}
	return nil
}

// typeForContext finds the type arg names, or the type of the file at arg,
// and says what it covers. A file that doesn't exist yet has no front matter
// to route on, so suggest names the types its front matter could pick.
func typeForContext(kit *voice.Kit, root, wd, arg string) (t *voice.Type, what, suggest string, err error) {
	if t, ok := kit.Types[arg]; ok {
		what = t.Doc
		if what == "" {
			what = t.Match.String()
		}
		return t, what, "", nil
	}

	rel, err := relToRoot(root, wd, arg)
	if err != nil {
		return nil, "", "", err
	}
	file := arg
	if !filepath.IsAbs(file) {
		file = filepath.Join(wd, file)
	}
	var fm voice.FrontMatter
	src, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		var lines []string
		for _, ft := range kit.FrontMatterTypes(rel) {
			lines = append(lines, fmt.Sprintf("- %s: run `plugh context %s` instead", ft.Match, ft.Name))
		}
		if len(lines) > 0 {
			suggest = "This file doesn't exist yet. If it'll have front matter that one of these types matches, use that type:\n\n" + strings.Join(lines, "\n")
		}
	case err != nil:
		return nil, "", "", err
	default:
		if fm, err = voice.ParseFrontMatter(src); err != nil {
			return nil, "", "", fmt.Errorf("%s: %w", arg, err)
		}
	}

	if t, err = kit.TypeFor(rel, fm); err != nil {
		return nil, "", "", err
	}
	if t == nil {
		if suggest != "" {
			return nil, "", "", fmt.Errorf("%s matches no type by its path alone. %s", rel, suggest)
		}
		return nil, "", "", fmt.Errorf("%s matches no type in %s; types are %s", arg, voice.RouterFile,
			strings.Join(slices.Sorted(maps.Keys(kit.Types)), ", "))
	}
	return t, t.Match.String(), suggest, nil
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
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		fm, err := voice.ParseFrontMatter(src)
		var t *voice.Type
		if err == nil {
			t, err = kit.TypeFor(rel, fm)
		}
		if err != nil { // a file that can't be routed is an error in it
			fmt.Fprintf(out, "%s: %v\n", file, err)
			errs++
			continue
		}
		if t == nil {
			continue // not content any type covers, like a readme
		}
		checks, err := kit.Checks(t)
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
