# plugh

A Claude Code plugin marketplace and a Go module in one repo, for writing in a project's own voice. For now it holds one plugin, also called `plugh`.

A project keeps its voice and tone guide as small chunks in a `voice/` folder, and a `voice.yaml` at its root says which chunks apply to which kind of content and who's writing for whom. That way the docs, the changelog and the README can each get the rules that fit them. Agents load those chunks before writing; the `plugh` command line tool prints the same context and runs the mechanical checks.

plugh ships no rules of its own. [examples/](examples/) is a whole made-up project to copy from.

## What's here

```
.claude-plugin/
  marketplace.json   this repo as a marketplace, listing the plugh plugin
  plugin.json        the plugin (the repo root is the plugin root)
bin/
  plugh              runs the real plugh, however it's installed (see below)
skills/
  write/             load the routed chunks, write, check
  review/            review a draft against them, without rewriting it
examples/            a made-up project: voice.yaml and voice/*.md
internal/voice/      loading, routing, validating and checking
cmd/plugh/           the command line tool
go.mod               github.com/stevenosloan/plugh
```

## The router

`voice.yaml` at the project root says who's writing, who's reading, and which chunks apply to which content. `plugh` finds it by walking up from the current folder, so the tool works from anywhere in the project. [examples/voice.yaml](examples/voice.yaml) has all of it in one file.

### Top level

| Key | Meaning |
| --- | --- |
| `plugh` | The plugh version the voice was written for, like `v1.0.1`. Optional. See [Pinning a version](#pinning-a-version) |
| `personas` | Who can write or read, by name. See [Personas](#personas) |
| `role` | The persona every type writes as, unless the type names its own |
| `audience` | The persona every type writes for, unless the type names its own |
| `types` | The kinds of content and jobs, by name. Each name is what `plugh context <type>` takes |

### Types

A type is either a kind of content, found by its path, or a job that isn't a file of its own, like writing a release note:

```yaml
types:
  guide:
    match: docs/guides/
    always: [voice-core, punctuation, guide]
    if_relevant: [code-samples]
    budget: 1000
  release-note:
    doc: the summary at the top of a GitHub release
    always: [voice-core, changelog]
```

| Field | Meaning |
| --- | --- |
| `match` | A path prefix, relative to the project root, like `docs/guides/` or `CHANGELOG.md`. A file belongs to the type with the longest prefix it starts with, so `docs/` and `docs/guides/` can both be types. Leave it out for a job |
| `doc` | For a job: what it's for, in a few words. `plugh context` prints it in place of the path |
| `always` | Chunk ids an agent reads every time, along with everything in their `depends_on`. Dependencies load first, and each chunk loads once. Checks come only from these chunks. Must not be empty |
| `if_relevant` | Chunk ids an agent sees only as a one-line `summary`, and loads with `plugh chunk <id>` if it applies, like a chunk on code samples for a guide that has some. Their checks don't run |
| `role`, `audience` | Persona names that override the top-level ones for this type |
| `budget` | The most words the type's context can run to: its `always` chunks plus the role and audience descriptions. `plugh kit` fails if it's over. It keeps the rules that load every time short enough to be followed. `0` or left out means no limit |

`plugh check` and `plugh context <file>` skip files that no type matches, like a README you don't want held to the guide.

### Personas

A persona is a short description of someone who writes or reads the content: who they are, what they know, and what they want from it.

```yaml
personas:
  maintainer:
    description: One of the people who builds tern...
  newcomer:
    description: Someone who installed tern today...
  regular:
    description: Someone who uses tern every week...
role: maintainer
audience: newcomer
types:
  reference:
    match: docs/reference/
    audience: regular
```

`plugh context` prints the role and audience ahead of the chunks. Both are optional: a type with neither gets only its chunks.

### Pinning a version

`plugh: v1.0.1` says the project's voice was written for plugh 1.0.1. Any later release that semver calls compatible works with it: 1.0.2 and 1.4.0 do, 1.0.0 and 2.0.0 don't. Below 1.0, a minor release can break things, so `v0.2.0` works with 0.2.x releases only. `plugh kit` warns if the installed plugh is older than the pin or on a different line. When plugh isn't installed, the plugin's `bin/plugh` runs the pinned version itself.

### What `plugh kit` checks

A `plugh` pin that isn't a version, a chunk id that doesn't exist, a `depends_on` cycle, a type with an empty `always`, a role or audience that isn't in `personas`, and a type over its budget. It reports all of them at once, along with any chunk that doesn't load.

## Chunks

A chunk is `voice/<id>.md`, a markdown file with YAML front matter:

| Field | Meaning |
| --- | --- |
| `id` | Matches the file name; what the router refers to |
| `summary` | One line. Agents read only this for `if_relevant` chunks |
| `depends_on` | Chunks that must load with this one |
| `status` | `draft` (inferred, needs confirming) or `accepted` |
| `checks` | Regex checks the Go tool runs: `id`, `pattern`, `message`, optional `severity` (`error` by default, or `warn`) |

Each rule in the body says the rule, **Why**, and **When unsure**. To change a rule, change the chunk, reason first, so the history says why.

## Use it in a project

1. Add the marketplace and turn the plugin on for that project only, in its `.claude/settings.json`:

   ```json
   {
     "extraKnownMarketplaces": {
       "plugh": { "source": { "source": "github", "repo": "stevenosloan/plugh" } }
     },
     "enabledPlugins": { "plugh@plugh": true }
   }
   ```

   To try a local copy instead, use `{ "source": "directory", "path": "/path/to/plugh" }` as the source.

2. Copy `examples/voice.yaml` and `examples/voice/` to the project root, then make them yours: the personas, the types, and the rules.

3. Install the `plugh` tool. It's one binary, and the project doesn't need to be a Go module:

   ```sh
   go install github.com/stevenosloan/plugh/cmd/plugh@<version>
   ```

   Set `plugh: <version>` in `voice.yaml` too. A Go project that wants plugh pinned in its own `go.mod` can run `go get -tool github.com/stevenosloan/plugh/cmd/plugh@<version>` instead.

   Installing is optional while the plugin is on. The plugin's [bin/plugh](bin/plugh) is on Claude's PATH, and it runs the first plugh it finds: one on PATH, then the project's Go tool, then `go run github.com/stevenosloan/plugh/cmd/plugh@<version>` with the version `voice.yaml` pins, or `latest`. That last one needs Go 1.27 or later (or `GOTOOLCHAIN=auto`, which fetches it). So the skills just run `plugh`.

## The plugh tool

Run it from anywhere inside a project with a `voice.yaml`:

```sh
plugh context docs/guides/install.md   # what an agent reads before writing
plugh context release-note             # same, for a job instead of a folder
plugh chunk code-samples               # one chunk, e.g. an if_relevant one
plugh check                            # check every markdown file a type covers
plugh check docs/guides                # or just some of them
plugh kit                              # check the router and chunks themselves
plugh version                          # this build's version; works anywhere
```

`check` prints `file:line:col id: message (chunk)` and exits 1 if there are errors; warnings don't fail it. It skips front matter, code (fenced, indented and inline) and URLs, and files no type matches.

In this repo, `go test ./...` runs everything, including `plugh kit` against the example project. What's left is in [todo.md](todo.md).
