package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func parseObject(args []string) (map[string]any, error) {
	if len(args) == 0 {
		return map[string]any{}, nil
	}
	if len(args) == 1 && strings.HasPrefix(strings.TrimSpace(args[0]), "[") {
		return nil, usageError("expected a JSON object, not an array")
	}
	out := map[string]any{}
	if len(args) == 1 && strings.HasPrefix(strings.TrimSpace(args[0]), "{") {
		if err := json.Unmarshal([]byte(args[0]), &out); err != nil {
			return nil, usageError("invalid JSON object: " + err.Error())
		}
		return out, nil
	}
	for _, arg := range args {
		key, raw, ok := strings.Cut(arg, "=")
		if !ok || key == "" {
			return nil, usageError(fmt.Sprintf("expected key=value or a JSON object, got %q", arg))
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			value = raw
		}
		out[key] = value
	}
	return out, nil
}

func parseFlags(args []string, takesValue map[string]bool, boolFlags map[string]bool) (map[string]string, map[string]bool, []string, error) {
	values, switches := map[string]string{}, map[string]bool{}
	var multi []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") {
			positional = append(positional, arg)
			continue
		}
		name, val, has := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if boolFlags[name] {
			if has {
				return nil, nil, nil, usageError("--" + name + " does not take a value")
			}
			switches[name] = true
			continue
		}
		if !takesValue[name] {
			return nil, nil, nil, unknown("--"+name, optionNames(takesValue, boolFlags))
		}
		if has {
			if name == "env" {
				multi = append(multi, val)
			} else {
				values[name] = val
			}
			continue
		}
		i++
		if i >= len(args) {
			return nil, nil, nil, usageError("--" + name + " needs a value")
		}
		val = args[i]
		if name == "env" {
			multi = append(multi, val)
		} else {
			values[name] = val
		}
	}
	if len(multi) > 0 {
		values["env"] = strings.Join(multi, "\x00")
	}
	return values, switches, positional, nil
}

func optionNames(values map[string]bool, switches map[string]bool) []string {
	out := make([]string, 0, len(values)+len(switches))
	for k := range values {
		out = append(out, "--"+k)
	}
	for k := range switches {
		out = append(out, "--"+k)
	}
	return out
}

func intFlag(name, value string, fallback int, minValue int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < minValue {
		return 0, usageError("--" + name + " must be an integer of at least " + strconv.Itoa(minValue))
	}
	return n, nil
}

func readPayload(args []string, input []byte) ([]byte, error) {
	if len(args) == 0 || len(args) == 1 && args[0] == "-" {
		return input, nil
	}
	if len(args) != 1 {
		return nil, usageError("provide one JSON object or '-' for stdin")
	}
	if strings.HasPrefix(strings.TrimSpace(args[0]), "{") {
		return []byte(args[0]), nil
	}
	obj, err := parseObject(args)
	if err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}

func formatList[T any](items []T, name func(T) string) string {
	var b strings.Builder
	for _, item := range items {
		fmt.Fprintln(&b, name(item))
	}
	return b.String()
}
