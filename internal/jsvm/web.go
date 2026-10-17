package jsvm

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

//go:embed prelude.js
var preludeJS string

// maxFetchBody bounds how much of a response body fetch reads.
const maxFetchBody = 32 << 20

// FetchResult is the raw outcome of one HTTP request.
type FetchResult struct {
	Status     int
	StatusText string
	URL        string
	Redirected bool
	Headers    map[string]string
	Body       string
}

// Fetch performs one HTTP request. Redirects are followed unless redirect is
// "manual". timeout 0 means no limit besides ctx.
func Fetch(ctx context.Context, timeout time.Duration, rawURL, method string, headers map[string]string, body string, hasBody bool, redirect string) (FetchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var rd io.Reader
	if hasBody {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return FetchResult{}, err
	}
	req.Header.Set("User-Agent", "meno")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	redirected := false
	client := &http.Client{Timeout: timeout}
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if redirect == "manual" {
			return http.ErrUseLastResponse
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		redirected = true
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBody))
	if err != nil {
		return FetchResult{}, err
	}
	res := FetchResult{
		Status:     resp.StatusCode,
		StatusText: strings.TrimSpace(strings.TrimPrefix(resp.Status, fmt.Sprint(resp.StatusCode))),
		URL:        resp.Request.URL.String(),
		Redirected: redirected,
		Headers:    map[string]string{},
		Body:       string(b),
	}
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		res.Headers[strings.ToLower(k)] = strings.Join(resp.Header.Values(k), ", ")
	}
	return res, nil
}

// ParseURL splits href (resolved against base when given) into WHATWG-style
// components. It returns nil for anything that is not an absolute URL.
func ParseURL(href, base string) map[string]any {
	var u *url.URL
	var err error
	if base != "" {
		var b *url.URL
		if b, err = url.Parse(base); err != nil || b.Scheme == "" {
			return nil
		}
		var r *url.URL
		if r, err = url.Parse(href); err != nil {
			return nil
		}
		u = b.ResolveReference(r)
	} else if u, err = url.Parse(href); err != nil {
		return nil
	}
	if u.Scheme == "" {
		return nil
	}
	proto := strings.ToLower(u.Scheme) + ":"
	special := map[string]bool{"http:": true, "https:": true, "ws:": true, "wss:": true, "ftp:": true, "file:": true}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	path := u.EscapedPath()
	if path == "" && special[proto] {
		path = "/"
	}
	out := map[string]any{
		"protocol":  proto,
		"username":  "",
		"password":  "",
		"hostname":  host,
		"port":      u.Port(),
		"pathname":  path,
		"search":    "",
		"hash":      "",
		"authority": u.Host != "" || special[proto],
	}
	if u.User != nil {
		out["username"] = u.User.Username()
		out["password"], _ = u.User.Password()
	}
	if u.RawQuery != "" {
		out["search"] = "?" + u.RawQuery
	}
	if f := u.EscapedFragment(); f != "" {
		out["hash"] = "#" + f
	}
	if u.Opaque != "" {
		out["pathname"] = u.Opaque
	}
	return out
}

// environ returns the process environment as a map.
func environ() map[string]any {
	m := map[string]any{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			m[k] = v
		}
	}
	return m
}
