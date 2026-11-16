package voice

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// FrontMatter is a markdown file's front matter, by top-level key.
type FrontMatter map[string]yaml.Node

// ParseFrontMatter reads the front matter at the top of src, the same block
// a chunk's settings go in. A file with no block has none, and that isn't an
// error. Unlike a chunk's, any keys will do.
func ParseFrontMatter(src []byte) (FrontMatter, error) {
	meta, _, err := splitFrontMatter(src)
	if err != nil {
		return nil, err
	}
	fm := FrontMatter{}
	if err := yaml.Unmarshal(meta, &fm); err != nil {
		return nil, fmt.Errorf("front matter: %w", err)
	}
	return fm, nil
}

// has says whether key is want, or a list holding want. Values compare as
// written, so draft: true has "true", and aliases count as what they name.
func (fm FrontMatter) has(key, want string) bool {
	node, ok := fm[key]
	if !ok {
		return false
	}
	is := func(n *yaml.Node) bool {
		n = resolve(n)
		return n.Kind == yaml.ScalarNode && n.Value == want
	}
	switch n := resolve(&node); n.Kind {
	case yaml.ScalarNode:
		return is(n)
	case yaml.SequenceNode:
		return slices.ContainsFunc(n.Content, is)
	}
	return false
}

// A Match says which files a type covers: those under Path, if it's set,
// whose front matter has every FrontMatter entry. A job has neither.
//
// In voice.yaml it's either a path, like docs/guides/, or a map with path
// and front_matter.
type Match struct {
	Path        string            `yaml:"path"`
	FrontMatter map[string]string `yaml:"front_matter"`

	given bool // written as a map, which then can't be empty
}

// UnmarshalYAML reads a match. Its errors are yaml.TypeErrors, so decoding
// carries on and reports the rest of voice.yaml's problems too.
func (m *Match) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Decode(&m.Path)
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i < len(n.Content); i += 2 {
			key, val := n.Content[i], resolve(n.Content[i+1])
			if err := once(seen, "match", key); err != nil {
				return err
			}
			switch key.Value {
			case "path":
				if val.Kind != yaml.ScalarNode {
					return typeErr(val, "match path is one path, like docs/guides/")
				}
				if err := val.Decode(&m.Path); err != nil {
					return typeErr(val, "match path: %v", err)
				}
			case "front_matter":
				if err := m.decodeFrontMatter(val); err != nil {
					return err
				}
			default:
				return typeErr(key, "match has %q; it takes path and front_matter", key.Value)
			}
		}
		m.given = true
		return nil
	}
	return typeErr(n, "match is a path, or a map with path and front_matter")
}

// decodeFrontMatter reads a match's front_matter, a map of keys to single
// values. Left empty, it's none.
func (m *Match) decodeFrontMatter(n *yaml.Node) error {
	if n.ShortTag() == "!!null" {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return typeErr(n, "match front_matter is a map of keys to values, like layout: tutorial")
	}
	m.FrontMatter = map[string]string{}
	seen := map[string]int{}
	for i := 0; i < len(n.Content); i += 2 {
		key, val := n.Content[i], resolve(n.Content[i+1])
		if err := once(seen, "match front_matter", key); err != nil {
			return err
		}
		// A file can write null as nothing, ~ or null, and has() compares
		// as written, so a null here couldn't match them all.
		if val.ShortTag() == "!!null" {
			return typeErr(val, "match front_matter %s has no value; give it one, like %s: tutorial", key.Value, key.Value)
		}
		var v string
		if val.Kind != yaml.ScalarNode || val.Decode(&v) != nil {
			return typeErr(val, "match front_matter %s is a single value, like %s: tutorial; it matches a list holding it",
				key.Value, key.Value)
		}
		m.FrontMatter[key.Value] = v
	}
	return nil
}

// once records key in seen, and errors if it's already there. A map decoded
// by hand skips yaml's own check for repeated keys.
func once(seen map[string]int, where string, key *yaml.Node) error {
	if first, ok := seen[key.Value]; ok {
		return typeErr(key, "%s has %s twice; the first is on line %d", where, key.Value, first)
	}
	seen[key.Value] = key.Line
	return nil
}

// typeErr is an error at n, in the form yaml collects to report together.
func typeErr(n *yaml.Node, format string, args ...any) error {
	return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: ", n.Line) + fmt.Sprintf(format, args...)}}
}

// resolve follows n if it's an alias.
func resolve(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}

// IsZero says whether m matches nothing, as for a job.
func (m Match) IsZero() bool { return m.Path == "" && len(m.FrontMatter) == 0 }

// String describes m for a person, like "docs/ with layout: tutorial".
func (m Match) String() string {
	var fm []string
	for _, k := range slices.Sorted(maps.Keys(m.FrontMatter)) {
		fm = append(fm, k+": "+m.FrontMatter[k])
	}
	switch {
	case len(fm) == 0:
		return m.Path
	case m.Path == "":
		return "front matter " + strings.Join(fm, ", ")
	default:
		return m.Path + " with " + strings.Join(fm, ", ")
	}
}

// equal says whether m and o match the same files.
func (m Match) equal(o Match) bool {
	return m.Path == o.Path && maps.Equal(m.FrontMatter, o.FrontMatter)
}

// fitsPath says whether file is under m's path; one with no path fits all.
func (m Match) fitsPath(file string) bool { return strings.HasPrefix(file, m.Path) }

func (m Match) matches(file string, fm FrontMatter) bool {
	if m.IsZero() || !m.fitsPath(file) {
		return false
	}
	for k, v := range m.FrontMatter {
		if !fm.has(k, v) {
			return false
		}
	}
	return true
}

// beats says whether m is more specific than o: more front matter entries,
// then a longer path. Neither beating the other is a tie.
func (m Match) beats(o Match) bool {
	if len(m.FrontMatter) != len(o.FrontMatter) {
		return len(m.FrontMatter) > len(o.FrontMatter)
	}
	return len(m.Path) > len(o.Path)
}
