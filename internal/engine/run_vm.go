package engine

import (
	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/mcp"
)

// newVM builds the js tool of a session that still works through js.
func (e *Engine) newVM(s *runState) *jsvm.VM {
	vm := jsvm.New(jsvm.Options{Root: s.cwd, Timeout: e.settings.BashTimeoutDuration()})
	vm.SetPolicy(jsvm.Policy{Only: s.modes.Only})
	vm.SetSessionHost(sessionHost{e, s})
	vm.SetWebSearch(webHost{e, s})
	vm.SetAsker(asker{e, s})
	vm.SetViewer(viewer{e, s})
	vm.SetTasks(taskHost{e, s})
	vm.SetMCP(mcpHost{e, s, mcp.ResultStore{Dir: e.paths.File("mcp-results")}})
	vm.SetMCPAdmin(mcpAdmin{e})
	vm.SetAgents(childHost{e, s, vm}, s.session.Kind == agent.KindChild)
	if s.goal != nil {
		vm.SetGoal(goalHost{s})
	}
	return vm
}
