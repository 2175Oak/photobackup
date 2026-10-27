package repository

import (
	"golang.org/x/text/unicode/norm"
	"photo-backup/model"
	"sort"
	"strings"
)

func searchText(s string) string { return norm.NFC.String(strings.ToLower(strings.TrimSpace(s))) }

// Rank locally: searching never sends a photo or query to the AI provider.
// Keep exact matches ahead of fuzzy matches, then prefer tags and fewer edits.
// Each query word may match a different tag/caption; selected tag filters stay
// exact and are applied by SQLite before this function runs.
func rankPhotos(photos []model.Photo, words []string) []model.Photo {
	queries := []string{}
	for _, w := range words {
		if q := searchText(w); q != "" {
			if len([]rune(q)) > 128 {
				return []model.Photo{}
			}
			queries = append(queries, q)
		}
	}
	if len(queries) == 0 {
		return photos
	}
	if len(queries) > 16 {
		return []model.Photo{}
	}
	type ranked struct {
		photo model.Photo
		score int
		exact bool
	}
	matches := []ranked{}
	for _, p := range photos {
		caption := searchText(p.Caption)
		tags := make([]string, len(p.Tags))
		for i, t := range p.Tags {
			tags[i] = searchText(t.Name)
		}
		score, allExact, matched := 0, true, true
		for _, q := range queries {
			best, exact := 0, false
			if strings.Contains(caption, q) {
				best = 80
				exact = true
			}
			for _, tag := range tags {
				if tag == q {
					best = max(best, 100)
					exact = true
				} else if strings.Contains(tag, q) {
					best = max(best, 90)
					exact = true
				}
			}
			if !exact {
				query := []rune(q)
				limit := 0
				if len(query) >= 3 {
					limit = 1
				}
				if len(query) >= 8 {
					limit = 2
				}
				if limit > 0 {
					for _, tag := range tags {
						if d := substringEdits(query, []rune(tag)); d <= limit {
							best = max(best, 60-10*d)
						}
					}
					if d := substringEdits(query, []rune(caption)); d <= limit {
						best = max(best, 45-10*d)
					}
				}
			}
			if best == 0 {
				matched = false
				break
			}
			score += best
			allExact = allExact && exact
		}
		if matched {
			matches = append(matches, ranked{photo: p, score: score, exact: allExact})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.exact != b.exact {
			return a.exact
		}
		return a.score > b.score
	})
	result := make([]model.Photo, 0, len(matches))
	for _, m := range matches {
		result = append(result, m.photo)
	}
	return result
}

// Optimal-string-alignment distance to the best substring. Free leading/trailing
// text supports Thai captions without spaces between words. Unicode code points
// preserve vowels/tone marks; missing, extra, replaced and adjacent swapped
// characters each cost one edit. Three rows bound memory to caption length.
func substringEdits(query, text []rune) int {
	if len(query) == 0 {
		return 0
	}
	if len(text) == 0 {
		return len(query)
	}
	older := make([]int, len(text)+1)
	previous := make([]int, len(text)+1)
	current := make([]int, len(text)+1)
	for i := 1; i <= len(query); i++ {
		current[0] = i
		for j := 1; j <= len(text); j++ {
			cost := 1
			if query[i-1] == text[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
			if i > 1 && j > 1 && query[i-1] == text[j-2] && query[i-2] == text[j-1] {
				current[j] = min(current[j], older[j-2]+1)
			}
		}
		older, previous, current = previous, current, older
	}
	best := len(query)
	for _, d := range previous {
		best = min(best, d)
	}
	return best
}
