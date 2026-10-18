package websearch

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// duckURL is a variable so tests can point it at a local server.
var duckURL = "https://html.duckduckgo.com/html/"

var (
	duckAnchor  = regexp.MustCompile(`(?s)<a[^>]*result__a[^>]*>.*?</a>`)
	duckHref    = regexp.MustCompile(`href="([^"]*)"`)
	duckSnippet = regexp.MustCompile(`(?s)<a[^>]*result__snippet[^>]*>(.*?)</a>`)
	htmlTag     = regexp.MustCompile(`(?s)<[^>]*>`)
)

// duckSearch uses the no-key HTML endpoint. Its markup is simple enough to
// read with patterns; a layout change returns "no results", never bad data.
func duckSearch(ctx context.Context, query string, count int) (Answer, error) {
	q := url.Values{}
	q.Set("q", query)
	b, err := fetch(ctx, "POST", duckURL, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "text/html",
	}, []byte(q.Encode()))
	if err != nil {
		return Answer{}, err
	}
	page := string(b)
	links := duckAnchor.FindAllString(page, -1)
	snips := duckSnippet.FindAllStringSubmatch(page, -1)
	var res []Result
	for i, a := range links {
		m := duckHref.FindStringSubmatch(a)
		if m == nil {
			continue
		}
		href := cleanURL(html.UnescapeString(m[1]))
		title := anchorText(a)
		snippet := ""
		if i < len(snips) {
			snippet = anchorText(snips[i][0])
		}
		if href != "" {
			res = append(res, Result{Title: title, URL: href, Snippet: snippet})
		}
		if len(res) >= count {
			break
		}
	}
	if len(res) == 0 {
		return Answer{}, fmt.Errorf("duckduckgo: no results")
	}
	return Answer{Results: res}, nil
}

// anchorText is the visible text of a matched anchor without its markup.
func anchorText(a string) string {
	lo := strings.Index(a, ">")
	hi := strings.LastIndex(a, "</a>")
	if lo < 0 || hi <= lo {
		return ""
	}
	return cleanText(a[lo+1 : hi])
}

// cleanURL unwraps the duckduckgo.com/l/?uddg= redirect links.
func cleanURL(href string) string {
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if strings.HasSuffix(u.Host, "duckduckgo.com") && u.Path == "/l/" {
		if target := u.Query().Get("uddg"); target != "" {
			return target
		}
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return href
}

// cleanText removes tags and entities and collapses whitespace.
func cleanText(s string) string {
	s = htmlTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
