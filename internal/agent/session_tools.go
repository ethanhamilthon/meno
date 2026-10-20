package agent

import (
	"meno/internal/llm"
)

func toolResult(t Turn) llm.Msg {
	res := t.Output
	if t.Err != "" {
		res += "\nERROR: " + t.Err
	}
	return llm.Msg{Role: llm.RoleTool, CallID: t.CallID, Text: res}
}

func (s *Session) userMsg(t Turn) llm.Msg {
	m := llm.Msg{Role: llm.RoleUser, Text: withSystem(t)}
	for _, a := range t.Attachments {
		m.Images = append(m.Images, llm.Image{N: a.N, Mime: a.Mime, Path: s.AttachmentPath(a)})
	}
	return m
}

func (s *Session) ToMsgs() []llm.Msg {
	all := s.Snapshot()
	return s.msgsOf(all, len(all))
}

// msgsOf renders the first n of all turns the way ToMsgs does: the summary
// first, then the turns. What a turn becomes depends on the turns after it only
// through spent (a big output is cut once the model has answered it), and that
// is read from all, so the messages of a head are the messages of the full
// request and a request built from a head shares the cache of the full one.
func (s *Session) msgsOf(all []Turn, n int) []llm.Msg {
	turns := all[:n]
	var out []llm.Msg
	if s.Summary != "" {
		out = append(out, llm.Msg{Role: llm.RoleUser, Text: "Summary of earlier conversation:\n" + s.Summary})
	}
	for i := 0; i < len(turns); i++ {
		t := turns[i]
		if s.legacyJSTurn(t) {
			out = append(out, ReplayJSTurns([]Turn{t})...)
			continue
		}
		if t.Replay != nil {
			out = append(out, *t.Replay)
			if t.CallID != "" {
				out = append(out, resultMsg(all, i))
			}
			for i+1 < len(turns) && t.ReplayGroup != "" && turns[i+1].ReplayGroup == t.ReplayGroup {
				i++
				if turns[i].CallID != "" {
					out = append(out, resultMsg(all, i))
				}
			}
			continue
		}
		switch {
		case t.Role == llm.RoleUser:
			out = append(out, s.userMsg(t))
		case t.Code != "" || t.CallID != "":
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text, CallID: t.CallID, Code: t.Code, ToolName: s.legacyToolName()})
			out = append(out, resultMsg(all, i))
		case t.Role == llm.RoleAssistant:
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text})
		}
	}
	omitOldImages(out, MaxRequestImages)
	return out
}

// withSystem is the text of a user turn as the model sees it: the system-event
// block, when there is one, then the user's words.
func withSystem(t Turn) string {
	if t.System == "" {
		return t.Text
	}
	return t.System + "\n\n" + t.Text
}
