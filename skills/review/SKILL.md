---
name: review
description: Use when asked to review a draft for voice and tone in a project with a voice.yaml. Comments on the draft against the routed voice chunks without rewriting it.
---

# Review a draft for voice

1. Load context exactly as the `plugh:write` skill does, steps 1 and 2.
2. If `go tool plugh check <file>` works, run it and list its findings first.
3. Read the draft against the loaded chunks. For each issue, give:
   - the line or a short quote,
   - the chunk and rule it falls under (`punctuation: Em dashes are a spaced ---`),
   - a one-line suggestion.
4. Don't rewrite the author's sentences, and don't fix anything unless asked. The voice is theirs; the review points at the rule.
5. Note judgment calls no chunk covers in a separate "Not in the guide" list. Those are candidates for new rules, not findings.
