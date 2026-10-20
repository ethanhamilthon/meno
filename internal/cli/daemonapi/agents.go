package daemonapi

import (
	"time"

	"meno/internal/cli"
)

type agentWire struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Task        string  `json:"task"`
	Result      *string `json:"result"`
	Error       string  `json:"error"`
	BlockReason string  `json:"blockReason"`
	TimedOut    bool    `json:"waitTimedOut"`
}

func (w agentWire) agent() cli.Agent {
	a := cli.Agent{ID: w.ID, Status: w.Status, Task: w.Task, Error: w.Error}
	if w.Result != nil {
		a.Result = *w.Result
	}
	if a.Error == "" {
		a.Error = w.BlockReason
	}
	return a
}

func (a *API) agentOp(op string, in map[string]any) (agentWire, error) {
	var w agentWire
	return w, a.post("agents", op, in, &w)
}

// AgentsSpawn starts a subagent.
func (a *API) AgentsSpawn(r cli.AgentSpawnRequest) (cli.Agent, error) {
	in := map[string]any{"task": r.Task}
	for k, v := range map[string]string{"cwd": r.CWD, "provider": r.Provider, "model": r.Model, "effort": r.Effort} {
		if v != "" {
			in[k] = v
		}
	}
	w, err := a.agentOp("spawn", in)
	if err == nil && w.Status == "" {
		w.Status = "starting"
	}
	return w.agent(), err
}

// AgentsList lists the subagents of the session.
func (a *API) AgentsList() ([]cli.Agent, error) {
	var raw []agentWire
	if err := a.get("agents", "list", "", &raw); err != nil {
		return nil, err
	}
	out := make([]cli.Agent, 0, len(raw))
	for _, w := range raw {
		out = append(out, w.agent())
	}
	return out, nil
}

// AgentStatus returns one subagent.
func (a *API) AgentStatus(id string) (cli.Agent, error) {
	w, err := a.agentOp("status", map[string]any{"id": id})
	return w.agent(), err
}

// AgentSend sends a message to a subagent.
func (a *API) AgentSend(id, text string) (cli.Agent, error) {
	w, err := a.agentOp("send", map[string]any{"id": id, "text": text})
	return w.agent(), err
}

// AgentWait waits for a subagent.
func (a *API) AgentWait(id string, timeout time.Duration) (cli.AgentWait, error) {
	w, err := a.agentOp("wait", map[string]any{"id": id, "timeout": int(timeout / time.Second)})
	return cli.AgentWait{Agent: w.agent(), TimedOut: w.TimedOut}, err
}

// AgentResult returns the result of a subagent.
func (a *API) AgentResult(id string) (cli.Agent, error) {
	w, err := a.agentOp("result", map[string]any{"id": id})
	return w.agent(), err
}

// AgentCancel stops a subagent.
func (a *API) AgentCancel(id string) (cli.Agent, error) {
	w, err := a.agentOp("cancel", map[string]any{"id": id})
	return w.agent(), err
}

// AgentReport is the report of a child to its parent.
func (a *API) AgentReport(text string) error {
	return a.post("agents", "report", map[string]any{"text": text}, nil)
}

// AgentBlock tells the parent that the child is stuck.
func (a *API) AgentBlock(reason string) error {
	return a.post("agents", "block", map[string]any{"reason": reason}, nil)
}
