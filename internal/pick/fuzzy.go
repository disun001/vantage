package pick

import (
	"sort"
	"strings"
	"unicode"
)

// score reports whether every rune of pattern appears in s in order (case
// insensitive) and how good the match is. Higher is better: consecutive runs
// and matches at the start or after a separator score more, and shorter
// strings win ties. An empty pattern matches everything with score 0.
func score(pattern, s string) (int, bool) {
	p := []rune(strings.ToLower(pattern))
	t := []rune(strings.ToLower(s))
	total, pi, prev := 0, 0, -2
	for i, r := range t {
		if pi == len(p) {
			break
		}
		if r != p[pi] {
			continue
		}
		total++
		if i == prev+1 {
			total += 3 // consecutive
		}
		if i == 0 || !unicode.IsLetter(t[i-1]) && !unicode.IsDigit(t[i-1]) {
			total += 2 // word start
		}
		prev = i
		pi++
	}
	if pi < len(p) {
		return 0, false
	}
	return total*10 - len(t), true
}

// filter returns the items matching pattern, best first. With an empty
// pattern it returns them all in their original order.
func filter(pattern string, items []string) []string {
	if pattern == "" {
		return items
	}
	type hit struct {
		s     string
		score int
	}
	var hits []hit
	for _, it := range items {
		if sc, ok := score(pattern, it); ok {
			hits = append(hits, hit{it, sc})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.s
	}
	return out
}
