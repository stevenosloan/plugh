---
id: code-samples
summary: "Commands and output in code blocks: copyable, with the output shown, no prompt characters."
depends_on: [voice-core]
status: draft
---

# Code samples

### Copyable commands

No `$` prompts, and no placeholders without a note saying what to replace them with.

Why: readers paste commands straight into a terminal. A `$` breaks the paste, and an unexplained `<path>` stops them cold.
When unsure: paste it into a fresh shell yourself.

### Show the output

After a command, show what it prints, in its own block, trimmed to the lines that matter.

Why: output tells the reader it worked, and is what they'll search for when it didn't.
