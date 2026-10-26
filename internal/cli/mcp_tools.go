package cli

import (
	"bytes"
	"encoding/json"
	"strings"
)

// mcpTools implements `meno mcp tools SERVER [TOOL]`. Without TOOL it lists one
// line per tool; with TOOL it prints that tool's full description and input
// schema, which is what a caller needs before `meno mcp call`.
func mcpTools(args []string, streams IO, api API, jsonOut bool) (int, error) {
	if len(args) < 1 || len(args) > 2 {
		return 0, usageError("usage: meno mcp tools SERVER [TOOL]")
	}
	if err := checkServer(api, args[0]); err != nil {
		return 0, err
	}
	items, err := api.MCPTools(args[0])
	if err != nil {
		return 0, err
	}
	if len(args) == 1 {
		return 0, emit(streams, jsonOut, items, formatList(items, func(x Tool) string { return x.Name + descriptionSuffix(firstLine(x.Description)) }))
	}
	names := make([]string, 0, len(items))
	for _, t := range items {
		if t.Name == args[1] {
			return 0, emit(streams, jsonOut, t, toolDetail(t))
		}
		names = append(names, t.Name)
	}
	return 0, unknown(args[1], names)
}

// firstLine is the first non-empty line of a tool description.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// dedent removes the indentation servers put in multi-line descriptions: the
// common indent of every line after the first, and blank lines at both ends.
func dedent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	indent := -1
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := len(l) - len(strings.TrimLeft(l, " \t")); indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		l = strings.TrimRight(l, " \t")
		if i > 0 && len(l) >= indent && indent > 0 {
			l = l[indent:]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

func toolDetail(t Tool) string {
	var b strings.Builder
	b.WriteString(t.Name + "\n")
	if d := dedent(t.Description); d != "" {
		b.WriteString("\n" + d + "\n")
	}
	b.WriteString("\n")
	if len(t.InputSchema) == 0 {
		b.WriteString("This tool takes no declared arguments.\n")
		return b.String()
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if enc.Encode(t.InputSchema) != nil {
		b.WriteString("The input schema could not be printed; use --json.\n")
		return b.String()
	}
	b.WriteString("Input schema:\n" + buf.String())
	return b.String()
}
