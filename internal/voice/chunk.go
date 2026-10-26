// Package voice loads a project's voice kit (its router and chunks), resolves
// which chunks apply to a file, and runs their checks over markdown.
package voice

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// A Chunk is one small piece of the guide, like voice/punctuation.md.
type Chunk struct {
	ID        string   `yaml:"id"`
	Summary   string   `yaml:"summary"`
	DependsOn []string `yaml:"depends_on"`
	Status    string   `yaml:"status"`
	Checks    []*Check `yaml:"checks"`

	Body   string `yaml:"-"` // the markdown after the front matter
	Source string `yaml:"-"` // where it was read from, like "voice/punctuation.md"
}

// Words is the chunk's length as an agent sees it, for a type's budget.
func (c *Chunk) Words() int { return len(strings.Fields(c.Body)) }

// A Check is a regular expression a chunk asks the checker to flag.
type Check struct {
	ID       string   `yaml:"id"`
	Pattern  string   `yaml:"pattern"`
	Message  string   `yaml:"message"`
	Severity Severity `yaml:"severity"`

	Chunk *Chunk         `yaml:"-"` // the chunk that declares it
	re    *regexp.Regexp // compiled Pattern
}

// Severity is how a finding counts: errors fail a check run, warnings don't.
type Severity string

const (
	Error Severity = "error"
	Warn  Severity = "warn"
)

// LoadChunks reads every dir/*.md in fsys. It returns what it could load
// along with every problem it found, so one bad chunk doesn't hide the rest.
func LoadChunks(fsys fs.FS, dir string) (map[string]*Chunk, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	chunks := map[string]*Chunk{}
	var errs []error
	for _, name := range names {
		c, err := loadChunk(fsys, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		chunks[c.ID] = c
	}
	return chunks, errors.Join(errs...)
}

func loadChunk(fsys fs.FS, name string) (*Chunk, error) {
	src, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	meta, body, err := splitFrontMatter(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	c := &Chunk{Body: string(body), Source: name}
	if err := yaml.Unmarshal(meta, c); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if want := strings.TrimSuffix(path.Base(name), ".md"); c.ID != want {
		return nil, fmt.Errorf("%s: id is %q, but the file name says %q", name, c.ID, want)
	}

	var errs []error
	for _, ch := range c.Checks {
		ch.Chunk = c
		if ch.Severity == "" {
			ch.Severity = Error
		}
		if ch.Severity != Error && ch.Severity != Warn {
			errs = append(errs, fmt.Errorf("%s: check %s: severity is %q, want error or warn", name, ch.ID, ch.Severity))
		}
		if ch.re, err = regexp.Compile(ch.Pattern); err != nil {
			errs = append(errs, fmt.Errorf("%s: check %s: %w", name, ch.ID, err))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// splitFrontMatter separates a leading "---" YAML block from the markdown
// body. A file without one is all body.
func splitFrontMatter(src []byte) (meta, body []byte, err error) {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(src, []byte("---\n"))
	if !ok {
		return nil, src, nil
	}
	if bytes.HasPrefix(rest, []byte("---\n")) {
		return nil, rest[len("---\n"):], nil
	}
	meta, body, ok = bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return nil, nil, errors.New("front matter has no closing ---")
	}
	return meta, body, nil
}
