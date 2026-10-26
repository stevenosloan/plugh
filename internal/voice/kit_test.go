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
		got, ok := k.TypeFor(file)
		if !ok || got.Name != want {
			t.Errorf("TypeFor(%q) = %v, want %s", file, got, want)
		}
	}
	if got, ok := k.TypeFor("readme.md"); ok {
		t.Errorf("TypeFor(readme.md) = %s, want no type", got.Name)
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
