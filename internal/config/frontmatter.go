package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// splitFrontmatter separates an optional "---" YAML header from the body. ok is
// false when there is no header or it is never closed (then it is not a header).
func splitFrontmatter(text string) (header, body string, ok bool) {
	text = strings.TrimPrefix(text, "\ufeff")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", text, false
	}
	for i := 1; i < len(lines); i++ {
		if t := strings.TrimRight(lines[i], "\r"); strings.TrimSpace(t) == "---" || t == "..." {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", text, false
}

// parseYAMLHeader decodes a header as a YAML mapping and flattens it to string
// values keyed by lower-case key. Scalars keep their text; nested mappings and
// sequences are not flattened into strings (they are skipped), except that a
// nested value never leaks into a sibling key.
func parseYAMLHeader(header string) (map[string]string, error) {
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(header), &raw); err != nil {
		return nil, err
	}
	meta := make(map[string]string, len(raw))
	for k, v := range raw {
		key := strings.ToLower(strings.TrimSpace(k))
		switch t := v.(type) {
		case nil:
			meta[key] = ""
		case string:
			meta[key] = strings.TrimSpace(t)
		case bool, int, int64, uint64, float64:
			meta[key] = fmt.Sprint(t)
		}
	}
	return meta, nil
}

// parseLenientHeader is the fallback for headers that are not valid YAML, such
// as an unquoted description that contains ": ". It reads "key: value" lines;
// an indented line continues the previous value.
func parseLenientHeader(header string) map[string]string {
	meta := map[string]string{}
	last := ""
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimRight(line, "\r")
		if last != "" && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			meta[last] = strings.TrimSpace(meta[last] + " " + strings.TrimSpace(line))
			continue
		}
		k, v, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		last = strings.ToLower(strings.TrimSpace(k))
		meta[last] = v
	}
	return meta
}

// ParseFrontmatterDiag is ParseFrontmatter plus a warning when the header is
// not valid YAML and the lenient fallback was used.
func ParseFrontmatterDiag(text string) (meta map[string]string, body string, warn string) {
	header, body, ok := splitFrontmatter(text)
	if !ok {
		return nil, body, ""
	}
	meta, err := parseYAMLHeader(header)
	if err == nil {
		return meta, body, ""
	}
	return parseLenientHeader(header), body, "frontmatter is not valid YAML; read leniently"
}

// ParseFrontmatter splits an optional "---" YAML header from the body of a
// markdown file and returns its scalar values. A header that is not valid YAML
// is read leniently as "key: value" lines.
func ParseFrontmatter(text string) (map[string]string, string) {
	meta, body, _ := ParseFrontmatterDiag(text)
	return meta, body
}
