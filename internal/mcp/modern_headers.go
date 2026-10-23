package mcp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// headerSpec is one x-mcp-header annotation: the argument at Path is sent as
// the HTTP header Mcp-Param-<Name>.
type headerSpec struct {
	Name string
	Path []string
	Type string
}

const headerKey = "x-mcp-header"

// headerSpecs validates the x-mcp-header annotations of an input schema
// (modern HTTP only). An invalid annotation makes the whole tool invalid.
func headerSpecs(schema json.RawMessage) ([]headerSpec, error) {
	if len(schema) == 0 {
		return nil, nil
	}
	var root any
	if json.Unmarshal(schema, &root) != nil {
		return nil, nil
	}
	var specs []headerSpec
	seen := map[string]bool{}
	var walk func(node any, path []string, reachable bool) error
	walk = func(node any, path []string, reachable bool) error {
		switch n := node.(type) {
		case map[string]any:
			if v, ok := n[headerKey]; ok {
				if !reachable || len(path) == 0 {
					return errors.New(headerKey + " is not on a statically reachable property")
				}
				name, _ := v.(string)
				typ, _ := n["type"].(string)
				if err := checkHeaderName(name); err != nil {
					return err
				}
				if typ != "string" && typ != "integer" && typ != "boolean" {
					return fmt.Errorf("%s must be on a string, integer or boolean property", headerKey)
				}
				if seen[strings.ToLower(name)] {
					return fmt.Errorf("duplicate %s value %q", headerKey, name)
				}
				seen[strings.ToLower(name)] = true
				specs = append(specs, headerSpec{Name: name, Path: append([]string(nil), path...), Type: typ})
			}
			for k, child := range n {
				if k == "properties" && reachable {
					props, _ := child.(map[string]any)
					for pn, pv := range props {
						if err := walk(pv, append(path, pn), true); err != nil {
							return err
						}
					}
					continue
				}
				if k == headerKey {
					continue
				}
				if err := walk(child, nil, false); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range n {
				if err := walk(child, nil, false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root, nil, true); err != nil {
		return nil, err
	}
	return specs, nil
}

func checkHeaderName(name string) error {
	if name == "" {
		return errors.New(headerKey + " must not be empty")
	}
	for _, r := range name {
		if r > 0x7e || r <= 0x20 || strings.ContainsRune("\"(),/:;<=>?@[\\]{}", r) {
			return fmt.Errorf("%s value %q is not a valid HTTP header name", headerKey, name)
		}
	}
	return nil
}

// encodeHeaderValue returns v as a header value: as is when it is plain
// visible ASCII, otherwise as the Base64 sentinel form.
func encodeHeaderValue(v string) string {
	plain := v == strings.TrimSpace(v) && !(strings.HasPrefix(v, "=?base64?") && strings.HasSuffix(v, "?="))
	for _, r := range v {
		if r != '\t' && (r < 0x20 || r > 0x7e) {
			plain = false
		}
	}
	if plain {
		return v
	}
	return "=?base64?" + base64.StdEncoding.EncodeToString([]byte(v)) + "?="
}

// paramHeaders builds the Mcp-Param-* headers for the arguments of a call.
// A missing or null value omits its header.
func paramHeaders(specs []headerSpec, args map[string]any) map[string]string {
	out := map[string]string{}
	for _, s := range specs {
		var cur any = args
		for _, p := range s.Path {
			m, ok := cur.(map[string]any)
			if !ok {
				cur = nil
				break
			}
			cur = m[p]
		}
		if cur == nil {
			continue
		}
		var text string
		switch v := cur.(type) {
		case string:
			text = v
		case bool:
			text = strconv.FormatBool(v)
		case int:
			text = strconv.Itoa(v)
		case int64:
			text = strconv.FormatInt(v, 10)
		case float64:
			if v != math.Trunc(v) || math.Abs(v) > 1<<53-1 {
				continue
			}
			text = strconv.FormatInt(int64(v), 10)
		default:
			continue
		}
		out["Mcp-Param-"+s.Name] = encodeHeaderValue(text)
	}
	return out
}
