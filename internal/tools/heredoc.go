package tools

import "strings"

// HeredocBody is the source range of one complete heredoc body. Byte offsets
// use the original command string.
type HeredocBody struct {
	Tag       string
	Body      string
	Start     int
	End       int
	StripTabs bool
}

// FindHeredocBodies finds complete <<TAG, <<'TAG', and <<-TAG bodies. It
// returns no ranges when syntax is incomplete or too ambiguous to fold safely.
func FindHeredocBodies(command string) []HeredocBody {
	lines := strings.SplitAfter(command, "\n")
	starts := lineStarts(lines)
	var bodies []HeredocBody
	for line := 0; line < len(lines); line++ {
		header := strings.TrimSuffix(strings.TrimSuffix(lines[line], "\n"), "\r")
		tags, ok := heredocTags(header)
		if !ok {
			return nil
		}
		for _, item := range tags {
			found := false
			for end := line + 1; end < len(lines); end++ {
				candidate := strings.TrimSuffix(strings.TrimSuffix(lines[end], "\n"), "\r")
				if item.strip {
					candidate = strings.TrimLeft(candidate, "\t")
				}
				if candidate != item.tag {
					continue
				}
				bodyStart, bodyEnd := starts[line+1], starts[end]
				bodies = append(bodies, HeredocBody{Tag: item.tag, Body: command[bodyStart:bodyEnd], Start: bodyStart, End: bodyEnd, StripTabs: item.strip})
				line, found = end, true
				break
			}
			if !found {
				return nil
			}
		}
	}
	return bodies
}

func lineStarts(lines []string) []int {
	starts := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		starts[i] = offset
		offset += len(line)
	}
	return starts
}

type heredocTag struct {
	tag   string
	strip bool
}

func heredocTags(header string) ([]heredocTag, bool) {
	var tags []heredocTag
	quote := byte(0)
	for i := 0; i < len(header); i++ {
		char := header[i]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == 39 || char == 34 {
			quote = char
			continue
		}
		if char == '#' {
			break
		}
		if char != '<' || i+1 >= len(header) || header[i+1] != '<' {
			continue
		}
		if i+2 < len(header) && header[i+2] == '<' {
			return nil, false
		}
		i += 2
		strip := false
		if i < len(header) && header[i] == '-' {
			strip = true
			i++
		}
		for i < len(header) && (header[i] == ' ' || header[i] == '\t') {
			i++
		}
		if i >= len(header) {
			return nil, false
		}
		quote = 0
		if header[i] == 39 || header[i] == 34 {
			quote = header[i]
			i++
		}
		start := i
		for i < len(header) && isTagChar(header[i]) {
			i++
		}
		if i == start {
			return nil, false
		}
		tag := header[start:i]
		if quote != 0 {
			if i >= len(header) || header[i] != quote {
				return nil, false
			}
			i++
			quote = 0
		}
		if i < len(header) && !isDelimiter(header[i]) {
			return nil, false
		}
		tags = append(tags, heredocTag{tag: tag, strip: strip})
		i--
	}
	if quote != 0 {
		return nil, false
	}
	return tags, true
}

func isTagChar(char byte) bool {
	return char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || strings.ContainsRune("_./:=@%+,-", rune(char))
}

func isDelimiter(char byte) bool {
	return char == ' ' || char == '\t' || char == ';' || char == '|' || char == '&'
}
