---
id: words
summary: "Fixture for the checker tests."
checks:
  - id: slash-alternatives
    pattern: '\b[A-Za-z]\w*/\w*[A-Za-z]\w*\b'
    message: name alternatives with "or", not a slash
    severity: warn
  - id: markdown-capitalized
    pattern: '\bMarkdown\b'
    message: '"markdown" is always lowercase'
  - id: ive-got
    pattern: "(?i)\\bI've got\\b"
    message: prefer "I have" over "I've got"
---
