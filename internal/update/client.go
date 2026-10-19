package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultRepo is the GitHub repository releases come from; MENO_REPO overrides it.
const DefaultRepo = "ethanhamilthon/meno"

// Client talks to GitHub. The zero value uses the real GitHub.
type Client struct {
	Repo    string       // owner/name
	API     string       // base of the GitHub API, for tests
	Base    string       // base of the release downloads, for tests
	Web     string       // base of the github.com pages, for tests
	HTTP    *http.Client // nil: a client with a 30 s timeout
	Version string       // the running version, sent in the User-Agent
}

func (c Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	if r := os.Getenv("MENO_REPO"); r != "" {
		return r
	}
	return DefaultRepo
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "meno/"+c.Version)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp, nil
}

// latestFromRedirect reads the newest tag from the redirect of the
// releases/latest page. Unlike the API it has no tight rate limit (60 requests
// an hour per address), which matters behind a shared address.
func (c Client) latestFromRedirect(ctx context.Context) (string, error) {
	web := c.Web
	if web == "" {
		web = "https://github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, web+"/"+c.repo()+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "meno/"+c.Version)
	hc := *c.http()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	tag := loc[strings.LastIndex(loc, "/")+1:]
	if resp.StatusCode/100 != 3 || !strings.Contains(loc, "/tag/") {
		return "", fmt.Errorf("%s: no redirect to a release (%s)", req.URL, resp.Status)
	}
	if _, ok := parse(tag); !ok {
		return "", fmt.Errorf("the latest release has an unreadable tag %q", tag)
	}
	return tag, nil
}

// Latest returns the tag of the newest published release, for example "v0.2.0".
// It reads the redirect of the releases page and falls back to the API.
func (c Client) Latest(ctx context.Context) (string, error) {
	if c.API == "" {
		if tag, err := c.latestFromRedirect(ctx); err == nil {
			return tag, nil
		}
	}
	api := c.API
	if api == "" {
		api = "https://api.github.com"
	}
	resp, err := c.get(ctx, api+"/repos/"+c.repo()+"/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", err
	}
	if _, ok := parse(rel.Tag); !ok {
		return "", fmt.Errorf("the latest release has an unreadable tag %q", rel.Tag)
	}
	return rel.Tag, nil
}

func (c Client) downloadBase(tag string) string {
	if c.Base != "" {
		return c.Base
	}
	return "https://github.com/" + c.repo() + "/releases/download/" + tag
}
