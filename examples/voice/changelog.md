---
id: changelog
summary: "Changelog entries and release notes: what changed for the reader, breaking changes first."
depends_on: [voice-core]
status: draft
---

# Changelog and release notes

### Write what changed for the reader

"`tern sync` no longer follows symlinks out of the repo" rather than "Refactored the link resolver".

Why: readers check the changelog to decide whether to upgrade and what will break, not to see how we spent the week.
When unsure: if a change has no effect a user could notice, leave it out.

### Breaking changes first, with the fix

Put breaking changes at the top, each with the one thing to do about it.

Why: the breaking change is the one entry everybody needs to read.

### Past tense, no hype

"Added", "Fixed", "Removed". No "exciting" or "huge".

Why: a list of facts is easy to scan, and a changelog isn't the place to sell anything.
