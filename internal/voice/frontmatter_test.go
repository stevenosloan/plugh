package voice

import (
	"strings"
	"testing"
)

func TestParseFrontMatter(t *testing.T) {
	for src, want := range map[string]string{
		"no front matter\n":                   "",
		"---\nlayout: x\n---\nbody\n":         "layout=x",
		"---\r\nlayout: x\r\n---\r\nbody\r\n": "layout=x",
		"---\nlayout: x\n---":                 "layout=x", // closed at the end of the file
		"---\n---\nbody\n":                    "",
		"---\n---":                            "", // empty, closed at the end of the file
		"---\n---\n":                          "",
		"---\n---more\n":                      "error: front matter has no closing",
		"text\n---\nlayout: x\n---\n":         "", // not at the top
		"---\nlayout: x\nbody\n":              "error: front matter has no closing",
		"---\nlayout: [x\n---\n":              "error: front matter",
		"---\n- a list\n---\n":                "error: front matter",
	} {
		fm, err := ParseFrontMatter([]byte(src))
		got := ""
		if err != nil {
			got = "error: " + err.Error()
		} else if n, ok := fm["layout"]; ok {
			got = "layout=" + n.Value
		}
		if want == "" && got != "" || want != "" && !strings.HasPrefix(got, want) {
			t.Errorf("ParseFrontMatter(%q) = %q, want %q", src, got, want)
		}
	}
}
