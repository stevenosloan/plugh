package voice

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A Finding is one place a check matched.
type Finding struct {
	Line, Col int // 1-based; Col counts bytes and points at the start of the match
	Check     *Check
}

func (f Finding) String() string {
	return fmt.Sprintf("%d:%d %s: %s (%s)", f.Line, f.Col, f.Check.ID, f.Check.Message, f.Check.Chunk.Source)
}

// CheckMarkdown runs checks over the prose in src. Front matter, code (fenced,
// indented and inline) and URLs are blanked out first, so a check only sees
// words a reader would read as prose, at their original columns.
func CheckMarkdown(src []byte, checks []*Check) []Finding {
	var findings []Finding
	for i, line := range proseLines(src) {
		for _, ch := range checks {
			for _, m := range ch.re.FindAllStringIndex(line, -1) {
				findings = append(findings, Finding{Line: i + 1, Col: m[0] + 1, Check: ch})
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return findings
}

// proseLines splits src into lines with everything that isn't prose replaced
// by spaces. It's a small state machine over lines: front matter, then normal
// lines, fenced code and indented code.
func proseLines(src []byte) []string {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	lines := strings.Split(string(src), "\n")

	const (
		prose = iota
		frontMatter
		fenced
		indented
	)
	state := prose
	if len(lines) > 0 && lines[0] == "---" {
		state = frontMatter
	}
	var fence string // the opening fence, like "```" or "~~~~"
	prevBlank := true

	out := make([]string, len(lines))
	for i, line := range lines {
		blank := strings.TrimSpace(line) == ""
		switch state {
		case frontMatter:
			out[i] = blankOut(line)
			if i > 0 && line == "---" {
				state = prose
				blank = true // the body starts fresh, as after a blank line
			}
		case fenced:
			out[i] = blankOut(line)
			if f := fenceOf(line); f != "" && f[0] == fence[0] && len(f) >= len(fence) && strings.TrimSpace(line) == f {
				state = prose
			}
		case indented:
			if blank || isIndented(line) {
				out[i] = blankOut(line)
				break
			}
			state = prose
			fallthrough
		case prose:
			switch {
			case fenceOf(line) != "":
				fence = fenceOf(line)
				state = fenced
				out[i] = blankOut(line)
			case prevBlank && isIndented(line):
				state = indented
				out[i] = blankOut(line)
			default:
				out[i] = maskInline(line)
			}
		}
		prevBlank = blank
	}
	return out
}

var fenceRE = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// fenceOf returns the fence a line opens or closes with, or "".
func fenceOf(line string) string {
	if m := fenceRE.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
}

var (
	linkURLRE  = regexp.MustCompile(`\]\([^)]*\)`)
	autolinkRE = regexp.MustCompile(`<[a-z]+://[^>]*>`)
	bareURLRE  = regexp.MustCompile(`\bhttps?://\S+`)
)

// maskInline blanks inline code, link targets and URLs within one line.
func maskInline(line string) string {
	b := []byte(line)
	maskInlineCode(b)
	for _, re := range []*regexp.Regexp{linkURLRE, autolinkRE, bareURLRE} {
		for _, m := range re.FindAllIndex(b, -1) {
			for j := m[0]; j < m[1]; j++ {
				b[j] = ' '
			}
		}
	}
	return string(b)
}

// maskInlineCode blanks code spans in place. A span opens with a run of
// backticks and closes at the next run of the same length, so “a ` b“ is
// one span. A run with no closing match is left alone, as markdown does.
func maskInlineCode(b []byte) {
	for i := 0; i < len(b); {
		if b[i] != '`' {
			i++
			continue
		}
		open := runLen(b, i)
		end := -1
		for j := i + open; j < len(b); {
			if b[j] != '`' {
				j++
				continue
			}
			n := runLen(b, j)
			if n == open {
				end = j + n
				break
			}
			j += n
		}
		if end < 0 {
			i += open
			continue
		}
		for j := i; j < end; j++ {
			b[j] = ' '
		}
		i = end
	}
}

// runLen counts the backticks starting at b[i].
func runLen(b []byte, i int) int {
	n := 0
	for i+n < len(b) && b[i+n] == '`' {
		n++
	}
	return n
}

// blankOut replaces every byte with a space, keeping the line's length so
// columns after it would still line up.
func blankOut(line string) string { return strings.Repeat(" ", len(line)) }
