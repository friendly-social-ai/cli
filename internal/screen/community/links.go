package community

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	// markdownLinkPattern matches markdown links and images, capturing image mark, text and URL.
	markdownLinkPattern = regexp.MustCompile(`(!?)\[([^\]]*)\]\((https?://[^)\s]+)\)`)
	// bareLinkPattern matches URLs written as plain text.
	bareLinkPattern = regexp.MustCompile(`https?://[^\s<>()\[\]]+`)
)

// link is a URL found in a post with a label to show for it.
type link struct {
	label string
	url   string
	image bool
}

// links returns links of text in order of appearance without duplicates: markdown links labelled by their text,
// images numbered, and plain URLs labelled by themselves.
func links(text string) []link {
	type found struct {
		at int
		link
	}

	var all []found
	var taken [][]int
	images := 0
	for _, m := range markdownLinkPattern.FindAllStringSubmatchIndex(text, -1) {
		label, url := text[m[4]:m[5]], text[m[6]:m[7]]
		image := m[3] > m[2]
		if image {
			images++
			label = fmt.Sprintf("image %d", images)
		}

		if strings.TrimSpace(label) == "" {
			label = url
		}

		all = append(all, found{at: m[0], link: link{label: label, url: url, image: image}})
		taken = append(taken, m[:2])
	}

	for _, m := range bareLinkPattern.FindAllStringIndex(text, -1) {
		inside := false
		for _, t := range taken {
			if m[0] >= t[0] && m[1] <= t[1] {
				inside = true
				break
			}
		}

		if !inside {
			url := strings.TrimRight(text[m[0]:m[1]], ".,;:!?'\"")
			all = append(all, found{at: m[0], link: link{label: url, url: url}})
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].at < all[j].at })

	var result []link
	seen := make(map[string]bool)
	for _, f := range all {
		if !seen[f.url] {
			seen[f.url] = true
			result = append(result, f.link)
		}
	}

	return result
}
