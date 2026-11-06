---
name: write
description: Use before writing or editing prose in a project that has a voice.yaml at its root (docs, posts, changelogs, descriptions, READMEs). Loads the voice chunks routed to that kind of content.
---

# Write in the project's voice

1. Find the content type.
   - Run `plugh context <file>` (or `plugh context <type>` for a job like `description`) and read what it prints. For each chunk under "Load if relevant" whose summary applies, run `plugh chunk <id>`. Skip to step 3.
   - If `plugh` isn't available, read `voice.yaml` at the project root. Pick the type whose `match` prefix the file path starts with, or the type named for the job.
2. Load the chunks by hand.
   - Find who you're writing as and for: the type's `role` and `audience`, or the top-level ones if the type doesn't name them. Read those entries under `personas`.
   - Read each chunk in the type's `always` list, plus each chunk's `depends_on`, from `voice/<id>.md` at the project root.
   - For each chunk in `if_relevant`, read only its `summary` and load it if it applies (for example `photo-captions` when the post has photos).
3. Write as the role persona, for the audience persona. Follow the loaded rules; where they're silent, follow their "When unsure" lines and `voice-core`.
4. Check. If `plugh` is available, run `plugh check <file>` and fix every error. Treat warnings as questions, not orders.
5. Don't invent rules. If you made a call no chunk covers, say so when you hand the text back, so the guide can grow.
