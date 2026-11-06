package voice

import (
	"strings"
	"testing"
	"testing/fstest"
)

func chunkFile(id, deps, body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("---\nid: " + id + "\ndepends_on: [" + deps + "]\n---\n" + body)}
}

func testKit(t *testing.T, router string) (*Kit, error) {
	t.Helper()
	return Load(fstest.MapFS{
		"voice.yaml":     {Data: []byte(router)},
		"voice/core.md":  chunkFile("core", "", "one two three"),
		"voice/punct.md": chunkFile("punct", "core", "four five"),
		"voice/blog.md":  chunkFile("blog", "core, punct", "six"),
		"voice/a.md":     chunkFile("a", "b", "a"),
		"voice/b.md":     chunkFile("b", "a", "b"),
	})
}

func TestTypeForPicksLongestMatch(t *testing.T) {
	k, err := testKit(t, `
types:
  content: {match: content/, always: [core]}
  blog: {match: content/blog/, always: [blog]}
  job: {always: [core]}
`)
	if err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		"content/blog/2026-10-12-x/index.md": "blog",
		"content/recipes/x/index.md":         "content",
		"./content/blog/x.md":                "blog",
	} {
		got, err := k.TypeFor(file, nil)
		if err != nil || got == nil || got.Name != want {
			t.Errorf("TypeFor(%q) = %v, %v; want %s", file, got, err, want)
		}
	}
	if got, err := k.TypeFor("readme.md", nil); got != nil || err != nil {
		t.Errorf("TypeFor(readme.md) = %v, %v; want no type", got, err)
	}
}

func TestResolveOrdersDependenciesFirst(t *testing.T) {
	k, err := testKit(t, `
types:
  blog: {match: content/blog/, always: [blog], if_relevant: [core, a]}
`)
	if err != nil {
		t.Fatal(err)
	}
	always, relevant, err := k.Resolve(k.Types["blog"])
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(always); got != "core punct blog" {
		t.Errorf("always = %s, want core punct blog", got)
	}
	if got := ids(relevant); got != "a" {
		t.Errorf("relevant = %s, want a (core is already loaded)", got)
	}
}

func TestValidateReportsEverything(t *testing.T) {
	k, err := testKit(t, `
types:
  cyclic: {always: [a]}
  missing: {always: [nope]}
  long: {always: [blog], budget: 3}
  empty: {match: x/}
`)
	if err != nil {
		t.Fatal(err)
	}
	k.Types["nobody"] = &Type{Name: "nobody", Always: []string{"core"}, Audience: "ghost"}
	err = k.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want errors")
	}
	for _, want := range []string{
		"type cyclic: depends_on cycle: a -> b -> a",
		`type missing: no chunk "nope"`,
		"type long: its chunks run 6 words, over its budget of 3",
		"type empty: always is empty",
		`type nobody: audience is "ghost", which isn't in personas`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() is missing %q; got:\n%v", want, err)
		}
	}
}

func TestPersonasForFallsBackToTheKit(t *testing.T) {
	k, err := testKit(t, `
personas:
  me: {description: the writer}
  guest: {description: a guest writer}
  dev: {description: a developer}
  cook: {description: a home cook}
role: me
audience: dev
types:
  blog: {always: [core]}
  recipe: {always: [core], audience: cook}
  guest: {always: [core], role: guest}
`)
	if err != nil {
		t.Fatal(err)
	}
	for typ, want := range map[string]string{"blog": "me dev", "recipe": "me cook", "guest": "guest dev"} {
		role, audience, err := k.PersonasFor(k.Types[typ])
		if err != nil {
			t.Fatal(err)
		}
		if got := role.Name + " " + audience.Name; got != want {
			t.Errorf("PersonasFor(%s) = %s, want %s", typ, got, want)
		}
	}
}

func TestLoadChunksReportsBadChunks(t *testing.T) {
	fsys := fstest.MapFS{
		"chunks/good.md":    chunkFile("good", "", "fine"),
		"chunks/renamed.md": chunkFile("other", "", "x"),
		"chunks/bad.md":     {Data: []byte("---\nid: bad\nchecks:\n  - id: x\n    pattern: '('\n    severity: loud\n---\n")},
	}
	chunks, err := LoadChunks(fsys, "chunks")
	if _, ok := chunks["good"]; !ok {
		t.Error("good chunk wasn't loaded alongside the bad ones")
	}
	for _, want := range []string{`id is "other", but the file name says "renamed"`, "check x: error parsing regexp", `severity is "loud"`} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("LoadChunks error is missing %q; got: %v", want, err)
		}
	}
}

func TestExampleKitIsValid(t *testing.T) {
	// The example project: what `plugh kit` checks.
	k, err := loadExample(t)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Validate(); err != nil {
		t.Error(err)
	}
}

func ids(chunks []*Chunk) string {
	var s []string
	for _, c := range chunks {
		s = append(s, c.ID)
	}
	return strings.Join(s, " ")
}

func TestValidatePlughPin(t *testing.T) {
	for pin, ok := range map[string]bool{
		"": true, "v0.1.0": true, "v1.2.3-rc.1": true,
		"1.0": false, "latest": false, "v1.2": false, "v01.2.3": false, "v1.2.3+build1": false,
	} {
		k, err := testKit(t, "plugh: \""+pin+"\"\ntypes:\n  job: {always: [core]}\n")
		if err != nil {
			t.Fatal(err)
		}
		if err := k.Validate(); (err == nil) != ok {
			t.Errorf("plugh: %q: Validate() = %v, want ok = %v", pin, err, ok)
		}
	}
}

func TestTypeForFrontMatter(t *testing.T) {
	k, err := testKit(t, `
types:
  content: {match: content/, always: [core]}
  blog: {match: content/blog/, always: [blog]}
  tutorial:
    match: {path: content/, front_matter: {layout: tutorial}}
    always: [core]
  draft:
    doc: a draft anywhere, which is a job too
    match: {front_matter: {draft: true}}
    always: [core]
  go-tutorial:
    match: {front_matter: {layout: tutorial, tags: go}}
    always: [core]
  review:
    match: {path: content/, front_matter: {draft: true}}
    always: [core]
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ file, fm, want string }{
		{"content/blog/x.md", "", "blog"},
		{"content/blog/x.md", "layout: post", "blog"},
		{"content/blog/x.md", "layout: tutorial", "tutorial"},                     // front matter beats a longer path
		{"content/blog/x.md", "layout: tutorial\ntags: [go, cli]", "go-tutorial"}, // a list holding the value
		{"notes/x.md", "draft: true", "draft"},                                    // a bool, compared as written
		{"content/x.md", "draft: true", "review"},                                 // same entries, longer path
		{"notes/x.md", "draft: \"true\"", "draft"},
		{"notes/x.md", "draft: false", ""},
		{"notes/x.md", "layout: tutorial", ""}, // tutorial needs content/
	} {
		fm, err := ParseFrontMatter([]byte("---\n" + tt.fm + "\n---\nbody\n"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := k.TypeFor(tt.file, fm)
		name := ""
		if got != nil {
			name = got.Name
		}
		if err != nil || name != tt.want {
			t.Errorf("TypeFor(%q, %q) = %q, %v; want %q", tt.file, tt.fm, name, err, tt.want)
		}
	}

	// review and tutorial each have one entry and the same path.
	fm, _ := ParseFrontMatter([]byte("---\nlayout: tutorial\ndraft: true\n---\n"))
	if _, err := k.TypeFor("content/x.md", fm); err == nil || !strings.Contains(err.Error(), "review and tutorial") {
		t.Errorf("TypeFor on a tie = %v, want an error naming both", err)
	}
}

func TestMatchShapes(t *testing.T) {
	for router, want := range map[string]string{
		"match: docs/": "",
		"match: {path: docs/, front_matter: {layout: x}}": "",
		"match: [docs/]":                        "match is a path, or a map",
		"match: {paht: docs/}":                  `match has "paht"`,
		"match: {front_matter: {tags: [a, b]}}": "single values",
	} {
		_, err := testKit(t, "types:\n  x:\n    always: [core]\n    "+router+"\n")
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: Load() = %v, want %q", router, err, want)
		}
	}
}

func TestValidateMatches(t *testing.T) {
	k, err := testKit(t, `
types:
  a: {match: {path: docs/, front_matter: {layout: x}}, always: [core]}
  b: {match: {path: docs/, front_matter: {layout: x}}, always: [core]}
  c: {match: {}, always: [core]}
  d: {match: docs/, always: [core]}
`)
	if err != nil {
		t.Fatal(err)
	}
	err = k.Validate()
	for _, want := range []string{"types a and b have the same match", "type c: match has neither"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() = %v, want %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "type d") {
		t.Errorf("Validate() = %v; d is fine", err)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	for router, want := range map[string]string{
		"typse: {}\n":                        "field typse not found",
		"types:\n  x: {alwasy: [core]}\n":    "field alwasy not found",
		"personas:\n  p: {descripton: hi}\n": "field descripton not found",
		"":                                   "",
		"types:\n  x: {always: [core], doc: d}\n": "",
	} {
		_, err := testKit(t, router)
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%q: Load() = %v, want %q", router, err, want)
		}
	}

	_, err := LoadChunks(fstest.MapFS{
		"voice/x.md": {Data: []byte("---\nid: x\ndepend_on: [y]\n---\nbody\n")},
	}, "voice")
	if err == nil || !strings.Contains(err.Error(), "field depend_on not found") {
		t.Errorf("LoadChunks() = %v, want the typo reported", err)
	}
}
