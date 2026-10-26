package voice

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// RouterFile is the router's name at a project's root.
const RouterFile = "voice.yaml"

// ChunkDir is where a project keeps its chunks.
const ChunkDir = "voice"

// A Kit is everything one project's voice runs on: its router and its chunks. Role and Audience name
// the personas every type writes as and for, unless a type names its own.
type Kit struct {
	Personas map[string]*Persona `yaml:"personas"`
	Role     string              `yaml:"role"`
	Audience string              `yaml:"audience"`
	Types    map[string]*Type    `yaml:"types"`
	Chunks   map[string]*Chunk   `yaml:"-"`
}

// A Persona is someone a piece is written as or for: who they are, what they
// know, what they want from it.
type Persona struct {
	Name        string `yaml:"-"`
	Description string `yaml:"description"`
}

// Words is the persona's length as an agent sees it, for a type's budget.
func (p *Persona) Words() int { return len(strings.Fields(p.Description)) }

// A Type is one kind of content, like blog posts, or a job, like writing a
// description. Match is a path prefix; a job has none.
type Type struct {
	Name       string   `yaml:"-"`
	Match      string   `yaml:"match"`
	Doc        string   `yaml:"doc"`
	Always     []string `yaml:"always"`
	IfRelevant []string `yaml:"if_relevant"`
	Role       string   `yaml:"role"`     // persona writing; the kit's role if empty
	Audience   string   `yaml:"audience"` // persona reading; the kit's audience if empty
	Budget     int      `yaml:"budget"`   // words, 0 for no limit
}

// Load reads project's voice.yaml and voice/*.md. As with LoadChunks, it
// returns what it could load along with every problem.
func Load(project fs.FS) (*Kit, error) {
	src, err := fs.ReadFile(project, RouterFile)
	if err != nil {
		return nil, err
	}
	k := &Kit{}
	if err := yaml.Unmarshal(src, k); err != nil {
		return nil, fmt.Errorf("%s: %w", RouterFile, err)
	}
	for name, t := range k.Types {
		t.Name = name
	}
	for name, p := range k.Personas {
		p.Name = name
	}

	k.Chunks, err = LoadChunks(project, ChunkDir)
	return k, err
}

// TypeFor returns the type whose Match is the longest prefix of file, a
// slash-separated path relative to the project root.
func (k *Kit) TypeFor(file string) (*Type, bool) {
	file = path.Clean(file)
	var best *Type
	for _, t := range k.Types {
		if t.Match == "" || !strings.HasPrefix(file, t.Match) {
			continue
		}
		if best == nil || len(t.Match) > len(best.Match) {
			best = t
		}
	}
	return best, best != nil
}

// Resolve returns the chunks t always loads, each after the chunks it depends
// on, and the chunks it offers if relevant (without their dependencies, since
// an agent only reads their summaries).
func (k *Kit) Resolve(t *Type) (always, relevant []*Chunk, err error) {
	seen := map[string]bool{}     // already in always
	visiting := map[string]bool{} // on the current path, to catch cycles
	var visit func(id string, from []string) error
	visit = func(id string, from []string) error {
		if seen[id] {
			return nil
		}
		trail := append(slices.Clone(from), id)
		if visiting[id] {
			return fmt.Errorf("type %s: depends_on cycle: %s", t.Name, strings.Join(trail, " -> "))
		}
		c, ok := k.Chunks[id]
		if !ok {
			if len(from) == 0 {
				return fmt.Errorf("type %s: no chunk %q", t.Name, id)
			}
			return fmt.Errorf("type %s: chunk %s depends on %q, which doesn't exist", t.Name, from[len(from)-1], id)
		}
		visiting[id] = true
		for _, dep := range c.DependsOn {
			if err := visit(dep, trail); err != nil {
				return err
			}
		}
		visiting[id] = false
		seen[id] = true
		always = append(always, c)
		return nil
	}

	for _, id := range t.Always {
		if err := visit(id, nil); err != nil {
			return nil, nil, err
		}
	}
	for _, id := range t.IfRelevant {
		c, ok := k.Chunks[id]
		if !ok {
			return nil, nil, fmt.Errorf("type %s: no chunk %q", t.Name, id)
		}
		if !seen[id] {
			relevant = append(relevant, c)
		}
	}
	return always, relevant, nil
}

// PersonasFor returns who t is written as and for, either of which is nil if
// neither t nor the kit names one.
func (k *Kit) PersonasFor(t *Type) (role, audience *Persona, err error) {
	find := func(field, own, kitWide string) (*Persona, error) {
		name := cmp.Or(own, kitWide)
		if name == "" {
			return nil, nil
		}
		p, ok := k.Personas[name]
		if !ok {
			return nil, fmt.Errorf("type %s: %s is %q, which isn't in personas", t.Name, field, name)
		}
		return p, nil
	}
	if role, err = find("role", t.Role, k.Role); err != nil {
		return nil, nil, err
	}
	if audience, err = find("audience", t.Audience, k.Audience); err != nil {
		return nil, nil, err
	}
	return role, audience, nil
}

// Checks returns the checks of the chunks t always loads.
func (k *Kit) Checks(t *Type) ([]*Check, error) {
	always, _, err := k.Resolve(t)
	if err != nil {
		return nil, err
	}
	var checks []*Check
	for _, c := range always {
		checks = append(checks, c.Checks...)
	}
	return checks, nil
}

// Validate reports every problem with the router: missing chunks and
// personas, cycles, and types over their budget.
func (k *Kit) Validate() error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(k.Types)) {
		t := k.Types[name]
		if len(t.Always) == 0 {
			errs = append(errs, fmt.Errorf("type %s: always is empty", name))
		}
		always, _, err := k.Resolve(t)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		role, audience, err := k.PersonasFor(t)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		words := 0
		for _, c := range always {
			words += c.Words()
		}
		for _, p := range []*Persona{role, audience} {
			if p != nil {
				words += p.Words()
			}
		}
		if t.Budget > 0 && words > t.Budget {
			errs = append(errs, fmt.Errorf("type %s: its chunks run %d words, over its budget of %d", name, words, t.Budget))
		}
	}
	return errors.Join(errs...)
}
