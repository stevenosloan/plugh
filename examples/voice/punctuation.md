---
id: punctuation
summary: "Sentence case headings, no exclamation marks, the serial comma."
depends_on: [voice-core]
status: accepted
checks:
  - id: exclamation
    pattern: '[A-Za-z]!(\s|$)'
    message: no exclamation marks; let the content carry the excitement
    severity: warn
  - id: double-space
    pattern: '[.?]  +\S'
    message: one space after a sentence
---

# Punctuation

### Sentence case headings

"Sync a second machine", not "Sync A Second Machine".

Why: sentence case is easier to scan and matches how the headings read aloud.
When unsure: capitalize the first word and proper nouns only. tern stays lowercase, even at the start.

### No exclamation marks

Why: docs that are excited about themselves read like marketing. A working command is exciting enough.
When unsure: end on a period.

### The serial comma

"Files, folders, and links."

Why: it removes the ambiguity in lists of flags or files, where it matters most.
