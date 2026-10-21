package web

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// FetchOptions bound one fetch. Zero values use the defaults.
type FetchOptions struct {
	Timeout   time.Duration // whole request, default 20s
	MaxBytes  int64         // decoded body read, default 2 MiB
	MaxText   int           // text returned, default 100000 bytes
	Redirects int           // default 5
}

func (o FetchOptions) norm() FetchOptions {
	if o.Timeout <= 0 {
		o.Timeout = 20 * time.Second
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 2 << 20
	}
	if o.MaxText <= 0 {
		o.MaxText = 100000
	}
	if o.Redirects <= 0 {
		o.Redirects = 5
	}
	return o
}

// Page is a fetched document reduced to readable text.
type Page struct {
	URL       string `json:"url"`       // as requested
	FinalURL  string `json:"final_url"` // after redirects
	Title     string `json:"title,omitempty"`
	Type      string `json:"content_type,omitempty"`
	Status    int    `json:"status"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
	Notice    string `json:"notice,omitempty"`
}

// Fetch downloads rawURL under the policy and returns its text. Provider keys
// and cookies are never sent: the client has no jar and adds no credentials.
func (s *Service) Fetch(ctx context.Context, rawURL string, o FetchOptions) (Page, error) {
	o = o.norm()
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Page{}, fmt.Errorf("only absolute http and https URLs can be fetched: %q", rawURL)
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", "meno-web/1")
	req.Header.Set("Accept", "text/html,text/plain,application/json;q=0.9,*/*;q=0.1")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	cl := s.client(o.Redirects)
	resp, err := cl.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()
	page := Page{URL: rawURL, FinalURL: resp.Request.URL.String(), Status: resp.StatusCode}
	typ, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	page.Type = typ
	if resp.StatusCode >= 400 {
		return page, fmt.Errorf("the server answered HTTP %d", resp.StatusCode)
	}
	if !textual(typ) {
		return page, fmt.Errorf("content type %q is not text", typ)
	}
	body, err := decode(resp)
	if err != nil {
		return page, err
	}
	raw, err := io.ReadAll(io.LimitReader(body, o.MaxBytes+1))
	if err != nil {
		return page, err
	}
	if int64(len(raw)) > o.MaxBytes {
		raw, page.Truncated = raw[:o.MaxBytes], true
	}
	text := strings.ToValidUTF8(string(raw), "\uFFFD")
	if typ == "text/html" || typ == "application/xhtml+xml" {
		page.Title, text = htmlText(text)
	}
	if len(text) > o.MaxText {
		text, page.Truncated = strings.ToValidUTF8(text[:o.MaxText], ""), true
	}
	page.Text = text
	if page.Truncated {
		page.Notice = "the text was truncated at the configured size"
	}
	return page, nil
}

func (s *Service) client(redirects int) *http.Client {
	tr := &http.Transport{
		DialContext:         s.Policy.dialer().DialContext,
		Proxy:               nil, // a proxy would dial the target on our behalf and skip the policy
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true, // decoded by hand below, so the size limit counts decoded bytes
	}
	return &http.Client{Transport: tr, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > redirects {
			return errors.New("too many redirects")
		}
		if r.URL.Scheme != "http" && r.URL.Scheme != "https" {
			return fmt.Errorf("%w: redirect to %s", ErrBlocked, r.URL.Scheme)
		}
		return nil
	}}
}

func decode(resp *http.Response) (io.Reader, error) {
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		return gzip.NewReader(resp.Body)
	case "deflate":
		return flate.NewReader(resp.Body), nil
	case "", "identity":
		return resp.Body, nil
	}
	return nil, fmt.Errorf("unsupported content encoding %q", resp.Header.Get("Content-Encoding"))
}

func textual(t string) bool {
	return t == "" || strings.HasPrefix(t, "text/") || strings.Contains(t, "json") || strings.Contains(t, "xml")
}

var (
	reDrop  = regexp.MustCompile(`(?is)<(script|style|noscript|svg|head)\b.*?</(script|style|noscript|svg|head)>`)
	reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reBlock = regexp.MustCompile(`(?i)</?(p|div|br|li|ul|ol|tr|h[1-6]|section|article|pre|table)\b[^>]*>`)
	reTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace = regexp.MustCompile(`[ \t\r\f]+`)
	reLines = regexp.MustCompile(`\n\s*\n+`)
)

// htmlText returns the page title and its visible text.
func htmlText(src string) (title, text string) {
	if m := reTitle.FindStringSubmatch(src); m != nil {
		title = strings.TrimSpace(html.UnescapeString(reTag.ReplaceAllString(m[1], "")))
	}
	t := reDrop.ReplaceAllString(src, " ")
	t = reBlock.ReplaceAllString(t, "\n")
	t = reTag.ReplaceAllString(t, "")
	t = html.UnescapeString(t)
	t = reSpace.ReplaceAllString(t, " ")
	t = reLines.ReplaceAllString(strings.TrimSpace(t), "\n\n")
	return title, t
}
