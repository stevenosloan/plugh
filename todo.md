# The Go tool

`cmd/plugh`, which reads a project's `voice.yaml` and `voice/` chunks.

## Done

- [x] **Load the kit.** `internal/voice/chunk.go` and `kit.go`: chunks and `voice.yaml` decoded with yaml.v3, chunks from the project's `voice/`. Loading reports every bad chunk at once with `errors.Join` instead of stopping at the first.
- [x] **Resolve a route.** `Kit.TypeFor` picks the longest matching prefix; `Kit.Resolve` walks `depends_on` depth first, dependencies before dependents, with a `visiting` set to catch cycles.
- [x] **`plugh kit`.** Missing chunks, cycles, ids that don't match their file name, bad patterns or severities, empty types, and types over budget.
- [x] **`Check`.** `internal/voice/check.go`: a line-based state machine blanks front matter, fenced and indented code, inline code spans and URLs, keeping columns, then runs each check's regex. Golden-file test in `internal/voice/testdata/check/`.
- [x] **Personas.** `personas` in `voice.yaml`, with a default `role` and `audience` that a type can override; `context` prints them and `kit` checks the names.
- [x] **No built-in library.** Each project brings its own chunks; `examples/` is a whole made-up project.
- [x] **`plugh context`, `plugh chunk` and `plugh check`.** `cmd/plugh`, with tests that run a whole fake project in a temp folder.
- [x] **A standalone binary.** `go install` or `go run …@<version>`, so a project doesn't need plugh in its `go.mod`, or to be a Go module at all. `plugh version` reads `debug.ReadBuildInfo`; the plugin's `bin/plugh` finds the real one (PATH, then `go tool`, then `go run` at the `plugh:` pin in `voice.yaml`), so the skills just say `plugh`; and `plugh kit` warns when the installed binary is older or on another semver line (major, or minor below v1), using `golang.org/x/mod/semver`.

## Next

- [ ] **`plugh hook`.** Read the PostToolUse hook's JSON from stdin, pull out the edited file's path, and run `check` on it if a type covers it. Print findings to stderr and exit 2 so Claude sees them. Then add `hooks/hooks.json` to the plugin, calling `"${CLAUDE_PLUGIN_ROOT}"/bin/plugh hook`, so it works however plugh is installed. Practice: `encoding/json` into a small struct, reading `os.Stdin`.
- [ ] **owner/repo names in link text.** `[tdewolff/minify](https://github.com/tdewolff/minify)` trips a check like `slash-alternatives` that looks for `a/b`. Options: skip link text that matches the end of its URL, or let a check list `allow` exceptions in its chunk.
- [ ] **Check the description too.** Front matter is skipped, so the `description` type's checks never run on a post's `description:`. One way: parse the front matter and check that field with the `description` type.
- [ ] **Lines or AST?** The checker scans lines itself. Walking goldmark's AST would get nested lists and odd fences exactly right; worth it if the line scanner starts missing things.
