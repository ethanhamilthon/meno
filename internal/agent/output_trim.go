package agent

import (
	"strconv"
	"unicode/utf8"

	"meno/internal/llm"
)

// A big output is sent whole to the model that asked for it. Once the model has
// answered it (its next response is in the history) the middle is cut: every
// later request would carry those bytes again, and the model has used them.
// Cutting at that moment costs the provider's prefix cache almost nothing,
// because the output is still near the end of the history.
const (
	trimAbove = 1 << 10 // outputs up to this size are never cut
	trimHead  = 768
	trimTail  = 256
)

// resultMsg is the tool result of turn i of all, cut when the model is done with it.
func resultMsg(all []Turn, i int) llm.Msg {
	t := all[i]
	if len(t.Output) > trimAbove && answered(all, i) {
		t.Output = trimOutput(t.Output)
	}
	return toolResult(t)
}

// answered reports whether the model has responded after the result of turn i:
// a later assistant turn that does not belong to the same batch of calls.
func answered(all []Turn, i int) bool {
	for _, n := range all[i+1:] {
		if n.Role == llm.RoleAssistant && (n.ReplayGroup == "" || n.ReplayGroup != all[i].ReplayGroup) {
			return true
		}
	}
	return false
}

// trimOutput keeps the start and the end of out and says what was left out.
func trimOutput(out string) string {
	head, tail := trimHead, len(out)-trimTail
	for head > 0 && !utf8.RuneStart(out[head]) {
		head--
	}
	for tail < len(out) && !utf8.RuneStart(out[tail]) {
		tail++
	}
	cut := tail - head
	return out[:head] + "\n[… " + strconv.Itoa(cut) + " bytes cut: you have already used this output. The first " + strconv.Itoa(head) +
		" and the last " + strconv.Itoa(len(out)-tail) + " bytes are kept. Run the call again with a narrower command, or write the full output to a file and read parts of it.]\n" + out[tail:]
}
