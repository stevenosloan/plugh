package voice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureChecks returns the checks of the named testdata/chunks.
func fixtureChecks(t *testing.T, ids ...string) []*Check {
	t.Helper()
	chunks, err := LoadChunks(os.DirFS("testdata"), "chunks")
	if err != nil {
		t.Fatal(err)
	}
	var checks []*Check
	for _, id := range ids {
		c, ok := chunks[id]
		if !ok {
			t.Fatalf("no fixture chunk %q", id)
		}
		checks = append(checks, c.Checks...)
	}
	return checks
}

// TestGolden checks each testdata/check/*.md against its .golden file, which
// lists the expected findings as "line:col id", one per line.
func TestGolden(t *testing.T) {
	checks := fixtureChecks(t, "dashes", "words")
	files, err := filepath.Glob("testdata/check/*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(file, ".md") + ".golden")
			if err != nil {
				t.Fatal(err)
			}
			var got strings.Builder
			for _, f := range CheckMarkdown(src, checks) {
				fmt.Fprintf(&got, "%d:%d %s\n", f.Line, f.Col, f.Check.ID)
			}
			if got.String() != string(want) {
				t.Errorf("findings:\n%s\nwant:\n%s", got.String(), want)
			}
		})
	}
}

func TestProseLines(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"plain prose is kept", "a -- b", []string{"a -- b"}},
		{"inline code", "x `a -- b` y", []string{"x          y"}},
		{"double-backtick code", "x ``a ` b`` y", []string{"x           y"}},
		{"link target", "[here](https://e.com/a--b) ok", []string{"[here                      ok"}},
		{"bare url", "see https://e.com/a--b now", []string{"see                    now"}},
		{"tilde fence", "~~~\na -- b\n~~~\nc", []string{"   ", "      ", "   ", "c"}},
		{"longer closing fence", "```\na\n````\nb", []string{"   ", " ", "    ", "b"}},
		{"shorter fence doesn't close", "````\n```\na\n````\nb", []string{"    ", "   ", " ", "    ", "b"}},
		{"indented code after a blank line", "p\n\n    a -- b\nq", []string{"p", "", "          ", "q"}},
		{"indented continuation isn't code", "p\n    a -- b", []string{"p", "    a -- b"}},
		{"front matter", "---\nt: a -- b\n---\nx", []string{"   ", "         ", "   ", "x"}},
		{"a --- rule later isn't front matter", "x\n---\ny", []string{"x", "---", "y"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proseLines([]byte(tt.src))
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("proseLines(%q)\n got %q\nwant %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestCheckMarkdownColumns(t *testing.T) {
	checks := fixtureChecks(t, "words")
	got := CheckMarkdown([]byte("first line\nI like Markdown and Hugo/11ty."), checks)
	var ids []string
	for _, f := range got {
		ids = append(ids, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Check.ID))
	}
	want := "2:8 markdown-capitalized, 2:21 slash-alternatives"
	if strings.Join(ids, ", ") != want {
		t.Errorf("got %v, want %s", ids, want)
	}
}
