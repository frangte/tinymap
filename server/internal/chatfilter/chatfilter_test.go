package chatfilter

import (
	"testing"

	"golang.org/x/text/unicode/norm"

	"saigon-noir/server/content"
)

func filter(t *testing.T) *Filter {
	t.Helper()
	w, err := content.LoadChatWords()
	if err != nil {
		t.Fatal(err)
	}
	return New(w)
}

func TestListedWordsAreMasked(t *testing.T) {
	f := filter(t)
	cases := map[string]string{
		"đm thằng này":       "** thằng này",
		"ĐỊT":                "***",
		"vãi lồn thật":       "*** *** thật",
		"dm may":             "** may",
		"vcl luôn":           "*** luôn",
		"what the fuck":      "what the ****",
		"FUUUUCK":            "*******",
		"sh1t happens":       "**** happens",
		"đ.ị.t":              "*.*.*",
		"f u c k you":        "* * * * you",
		"du ma no":           "** ** no",
		"lồn":                "***",
		"bạn là con đĩ à":    "bạn là *** ** à",
		"đụ má, đi chơi đi!": "** **, đi chơi đi!",
	}
	for in, want := range cases {
		if got := f.Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLookAlikesStayReadable(t *testing.T) {
	f := filter(t)
	for _, s := range []string{
		"các bạn ơi", "cac ban oi", "lon bia", "buổi sáng", "buoi sang", "đeo kính", "deo kinh", "đi đâu đó", "di dau do",
		"con cá", "Lan ơi", "dâu tây", "đức bà", "con dê", "100 xu", "fun", "class", "dumb", "hello world",
	} {
		if got := f.Clean(s); got != s {
			t.Errorf("Clean(%q) = %q: an ordinary line was masked", s, got)
		}
	}
}

func TestDecomposedTextIsCaughtToo(t *testing.T) {
	f := filter(t)
	if got := f.Clean(norm.NFD.String("lồn")); got != "***" {
		t.Fatalf("decomposed: %q", got)
	}
}

func TestTheZeroFilterMasksNothing(t *testing.T) {
	var f *Filter
	if got := f.Clean("địt"); got != "địt" {
		t.Fatalf("nil filter: %q", got)
	}
}
