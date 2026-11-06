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

	"golang.org/x/mod/semver"
)

// RouterFile is the router's name at a project's root.
const RouterFile = "voice.yaml"

// ChunkDir is where a project keeps its chunks.
const ChunkDir = "voice"

// A Kit is everything one project's voice runs on: its router and its chunks. Role and Audience name
// the personas every type writes as and for, unless a type names its own.
// Plugh is the plugh version the project expects, like v0.1.0, if it pins one.
type Kit struct {
	Plugh    string              `yaml:"plugh"`
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
// description. Match says which files it covers; a job has none, though a
// type can be both, so a job and its files share chunks.
type Type struct {
	Name       string   `yaml:"-"`
	Match      Match    `yaml:"match"`
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
	if err := decodeStrict(src, k); err != nil {
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

// TypeFor returns the type that covers file, a slash-separated path
// relative to the project root, with front matter fm. Of the types that
// match, the one with the most front matter entries wins, then the longest
// path. It returns nil if none match, and an error if two tie.
func (k *Kit) TypeFor(file string, fm FrontMatter) (*Type, error) {
	file = path.Clean(file)
	var best *Type
	var tied []string
	for _, name := range slices.Sorted(maps.Keys(k.Types)) {
		t := k.Types[name]
		switch {
		case !t.Match.matches(file, fm):
		case best == nil || t.Match.beats(best.Match):
			best, tied = t, []string{name}
		case !best.Match.beats(t.Match):
			tied = append(tied, name)
		}
	}
	if len(tied) > 1 {
		return nil, fmt.Errorf("%s matches types %s equally; give one a longer path or more front_matter",
			file, strings.Join(tied, " and "))
	}
	return best, nil
}

// FrontMatterTypes returns the types with front matter in their match whose
// path file is under, for a file that doesn't exist yet: which one it gets
// depends on what front matter it's written with.
func (k *Kit) FrontMatterTypes(file string) []*Type {
	file = path.Clean(file)
	var types []*Type
	for _, name := range slices.Sorted(maps.Keys(k.Types)) {
		if t := k.Types[name]; len(t.Match.FrontMatter) > 0 && t.Match.fitsPath(file) {
			types = append(types, t)
		}
	}
	return types
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

// Validate reports every problem with the router: a malformed plugh pin,
// empty or duplicate matches, missing chunks and personas, cycles, and types over their budget.
func (k *Kit) Validate() error {
	var errs []error
	// Canonical drops build metadata (+meta) and fills in a short v1.2, so
	// only a full version Go can fetch comes back unchanged.
	if k.Plugh != "" && semver.Canonical(k.Plugh) != k.Plugh {
		errs = append(errs, fmt.Errorf("plugh is %q; want a version like v0.1.0", k.Plugh))
	}
	matches := map[string]string{} // Match.String to the type with it
	for _, name := range slices.Sorted(maps.Keys(k.Types)) {
		t := k.Types[name]
		if t.Match.given && t.Match.IsZero() {
			errs = append(errs, fmt.Errorf("type %s: match has neither path nor front_matter", name))
		} else if m := t.Match.String(); m != "" {
			if other, ok := matches[m]; ok {
				errs = append(errs, fmt.Errorf("types %s and %s have the same match, %s", other, name, m))
			}
			matches[m] = name
		}
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
