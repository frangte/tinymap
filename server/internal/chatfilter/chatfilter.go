// Package chatfilter masks words from chat lines and names (content/chat_filter.json; docs/design/04-backend.md,
// Moderation). Vietnamese makes "ignore the accents" a trap: many swear words lose their marks and become ordinary
// words (lồn/lon, cặc/các, buồi/buổi, đéo/đeo). So a word written with marks is matched as written, against the
// accented list; a word typed without marks is matched against a short plain list of forms nobody types by
// accident. Letters spread apart (đ.ị.t), stretched (fuuuck) or spelled with digits (sh1t) are caught as well.
package chatfilter

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"saigon-noir/server/content"
)

// longest is the most words a listed phrase has.
const longest = 3

// Filter masks listed words. Its zero value masks nothing.
type Filter struct {
	accented map[string]bool
	plain    map[string]bool
}

// New compiles the lists.
func New(w content.ChatWords) *Filter {
	f := &Filter{accented: map[string]bool{}, plain: map[string]bool{}}
	for _, s := range w.Accented {
		f.accented[norm.NFC.String(strings.ToLower(s))] = true
	}
	for _, s := range w.Plain {
		f.plain[strings.ToLower(s)] = true
	}
	return f
}

// word is a run of letters (and digits, which may stand for letters) in the line, by rune index.
type word struct {
	start, end int
	text       string
}

// Clean returns the line with every listed word or phrase masked by asterisks (same length, so nothing shifts).
func (f *Filter) Clean(s string) string {
	if f == nil || (len(f.accented) == 0 && len(f.plain) == 0) {
		return s
	}
	runes := []rune(norm.NFC.String(s))
	words := joinSpread(split(runes), runes)
	masked := false
	for i := range words {
		for n := min(longest, len(words)-i); n >= 1; n-- {
			if f.listed(words[i : i+n]) {
				for _, w := range words[i : i+n] {
					for k := w.start; k < w.end; k++ {
						if isWordRune(runes[k]) {
							runes[k] = '*'
						}
					}
				}
				masked = true
				break
			}
		}
	}
	if !masked {
		return s
	}
	return string(runes)
}

// listed says whether the words, as a phrase, are on a list.
func (f *Filter) listed(ws []word) bool {
	parts := make([]string, len(ws))
	for i, w := range ws {
		parts[i] = strings.ToLower(w.text)
	}
	for _, phrase := range []string{strings.Join(parts, " "), squeeze(strings.Join(parts, " "))} {
		if f.accented[phrase] {
			return true
		}
		if plain, ok := plainForm(phrase); ok && (f.plain[plain] || f.plain[squeeze(plain)]) {
			return true
		}
	}
	return false
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// split cuts the line into words.
func split(runes []rune) []word {
	var out []word
	start := -1
	for i, r := range runes {
		if isWordRune(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, word{start: start, end: i, text: string(runes[start:i])})
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, word{start: start, end: len(runes), text: string(runes[start:])})
	}
	return out
}

// joinSpread joins runs of one-letter words with at most two marks between them (đ.ị.t, f u c k) into one word.
func joinSpread(ws []word, runes []rune) []word {
	var out []word
	for i := 0; i < len(ws); {
		j := i
		for j+1 < len(ws) && ws[j].end-ws[j].start == 1 && ws[j+1].end-ws[j+1].start == 1 && ws[j+1].start-ws[j].end <= 2 {
			j++
		}
		if j-i >= 2 {
			var b strings.Builder
			for _, w := range ws[i : j+1] {
				b.WriteString(w.text)
			}
			out = append(out, word{start: ws[i].start, end: ws[j].end, text: b.String()})
		} else {
			out = append(out, ws[i:j+1]...)
		}
		i = j + 1
	}
	return out
}

// squeeze shortens runs of a repeated letter to one (fuuuck → fuck, lồnnn → lồn).
func squeeze(s string) string {
	var b strings.Builder
	var last rune = -1
	for _, r := range s {
		if r != last {
			b.WriteRune(r)
		}
		last = r
	}
	return b.String()
}

// leet maps digits and signs that stand for letters.
var leet = map[rune]rune{'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '@': 'a', '$': 's'}

// plainForm is the phrase as the plain list writes it: đ as d, digits as letters; ok is false when a letter has
// any other mark (a word written with marks is matched as written, never as its plain look-alike).
func plainForm(s string) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 'đ':
			b.WriteRune('d')
		case r < utf8.RuneSelf:
			if l, ok := leet[r]; ok {
				r = l
			}
			b.WriteRune(r)
		default:
			return "", false
		}
	}
	return b.String(), true
}
