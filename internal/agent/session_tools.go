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
	var out []llm.Msg
	if s.Summary != "" {
		out = append(out, llm.Msg{Role: llm.RoleUser, Text: "Summary of earlier conversation:\n" + s.Summary})
	}
	turns := s.Snapshot()
	for i := 0; i < len(turns); i++ {
		t := turns[i]
		if t.Replay != nil {
			out = append(out, *t.Replay)
			if t.CallID != "" {
				out = append(out, toolResult(t))
			}
			for i+1 < len(turns) && t.ReplayGroup != "" && turns[i+1].ReplayGroup == t.ReplayGroup {
				i++
				if turns[i].CallID != "" {
					out = append(out, toolResult(turns[i]))
				}
			}
			continue
		}
		switch {
		case t.Role == llm.RoleUser:
			out = append(out, s.userMsg(t))
		case t.Code != "" || t.CallID != "":
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text, CallID: t.CallID, Code: t.Code})
			out = append(out, toolResult(t))
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
