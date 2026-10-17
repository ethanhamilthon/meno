package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
)

type CacheOptions struct {
	Key       string
	Retention string
	TTL       string
	Mode      string
	Disabled  bool
}

var cacheSnapshot = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

func cacheModel(model string) (known, modern, extended bool) {
	model = cacheSnapshot.ReplaceAllString(model, "")
	switch model {
	case "gpt-5.6", "gpt-6", "gpt-6-astra", "gpt-6-sol", "gpt-6.1", "gpt-6.1-sol":
		return true, true, false
	case "gpt-5.5", "gpt-5.5-pro", "gpt-5.4", "gpt-5.2", "gpt-5.1",
		"gpt-5.1-codex-max", "gpt-5.1-codex", "gpt-5.1-codex-mini", "gpt-5.1-chat-latest",
		"gpt-5", "gpt-5-codex", "gpt-4.1":
		return true, false, true
	case "gpt-4o", "gpt-4o-mini", "gpt-4.1-mini", "gpt-4.1-nano",
		"gpt-5-mini", "gpt-5-nano", "o1", "o1-mini", "o3", "o3-mini", "o4-mini":
		return true, false, false
	}
	return false, false, false
}

func applyCache(body map[string]any, req Request) {
	u, err := url.Parse(req.Provider.BaseURL)
	official := err == nil && strings.EqualFold(u.Hostname(), "api.openai.com")
	if !official && req.Cache == nil {
		return
	}
	options := CacheOptions{}
	if req.Cache != nil {
		options = *req.Cache
	}
	if options.Disabled {
		return
	}
	known, modern, extended := cacheModel(req.Model)
	if !known {
		if req.Cache == nil {
			return
		}
		if options.Key != "" && len(options.Key) <= 64 {
			body["prompt_cache_key"] = options.Key
		}
		if options.TTL != "" || options.Mode != "" {
			modern = true
		} else {
			if options.Retention == "in_memory" || options.Retention == "24h" {
				body["prompt_cache_retention"] = options.Retention
			}
			return
		}
	}
	if !modern {
		key := options.Key
		if key == "" && req.System != "" {
			prefix := req.Model + "\x00" + req.System
			if req.Tools {
				prefix += "\x00" + mustJSON(ToolSchema(req.ModelTimeout))
			}
			sum := sha256.Sum256([]byte(prefix))
			key = "meno-" + hex.EncodeToString(sum[:])[:48]
		}
		if key != "" && len(key) <= 64 {
			body["prompt_cache_key"] = key
		}
		retention := options.Retention
		if retention == "" && extended {
			retention = "24h"
		}
		if retention == "in_memory" && req.Model != "gpt-5.5" && req.Model != "gpt-5.5-pro" && !strings.HasPrefix(req.Model, "gpt-5.5-") || retention == "24h" && extended {
			body["prompt_cache_retention"] = retention
		}
		return
	}
	ttl := options.TTL
	if ttl == "" {
		ttl = "30m"
	}
	mode := options.Mode
	if mode == "" {
		mode = "implicit"
	}
	if ttl == "30m" && mode == "implicit" {
		body["prompt_cache_options"] = map[string]any{"ttl": ttl, "mode": mode}
	}
	if options.Key != "" && len(options.Key) <= 64 {
		body["prompt_cache_key"] = options.Key
	}
}
