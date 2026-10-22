package render

// preserveWords wraps without deleting boundary spaces or changing styles.
func preserveWords(cells []wcell, width int) [][]wcell {
	width = max(1, width)
	var lines [][]wcell
	for start := 0; start < len(cells); {
		end, col, boundary := start, 0, start
		for end < len(cells) && !cells[end].nl {
			c := cells[end]
			if col+c.w > width && end > start {
				break
			}
			col += c.w
			end++
			if c.space {
				boundary = end
			}
		}
		if end < len(cells) && !cells[end].nl && boundary > start {
			end = boundary
		}
		lines = append(lines, cells[start:end])
		start = end
		if start < len(cells) && cells[start].nl {
			start++
			if start == len(cells) {
				lines = append(lines, nil)
			}
		}
	}
	if len(lines) == 0 {
		return [][]wcell{nil}
	}
	return lines
}
