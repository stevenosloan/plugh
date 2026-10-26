---
id: voice-core
summary: "How tern's docs sound: plain, direct, specific, and never 'simply'."
status: accepted
checks:
  - id: minimizers
    pattern: '(?i)\b(simply|just|easily|obviously)\b'
    message: words like "simply" tell the reader it's easy; if it isn't for them, it stings
    severity: warn
  - id: latin-abbreviations
    pattern: '\b(e\.g\.|i\.e\.)'
    message: write "for example" or "that is"
---

# Voice core

The defaults every other chunk builds on.

### Talk to the reader

Write "you" for the reader and "we" for the maintainers. Use the imperative for steps: "Run `tern sync`", not "The user should run `tern sync`".

Why: docs are a conversation with one person at a terminal, not a spec.
When unsure: read it aloud as if explaining it at a desk. If you wouldn't say it, rewrite it.

### Specific over general

Name the command, the flag, the file and the number. "Syncs in about two seconds for 200 files" beats "fast".

Why: specifics are what a reader can check and act on.
When unsure: ask "which one?" of every general noun, and answer it.

### Don't say it's easy

Leave out "simply", "just", "easily" and "obviously".

Why: they add nothing for a reader who finds it easy, and make one who doesn't feel slow.
When unsure: delete the word. The sentence almost always still works.
