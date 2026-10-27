package tui

import (
	"errors"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
	"meno/internal/mcp"
)

// The add wizard asks for one field of a new provider or MCP server at a time.
// The answer is typed in the input row of the panel; the fields and their
// rules are the ones of the edit forms (providerFields, mcpFields).
type wizard struct {
	mcp bool // false: a provider
	i   int  // the current step
}

// Fields that may stay empty, with what the empty input says.
var wizardOptional = map[string]string{
	"run":  "optional · enter to skip",
	"args": "optional · space separated · enter to skip",
}

func (a *App) startProviderWizard(p llm.Provider) {
	a.st.EditProv, a.st.EditIdx = p, -1
	a.openOverlay(OverWizard)
	a.st.Wiz = wizard{}
	a.loadStep()
}

func (a *App) startMCPWizard() {
	a.st.EditMCP, a.st.EditIdx = mcp.Server{Transport: "stdio", Enabled: true}, -1
	a.openOverlay(OverWizard)
	a.st.Wiz = wizard{mcp: true}
	a.loadStep()
}

// wizSteps are the labels of the fields asked, in order. An MCP server asks
// for a command and arguments over stdio, and for a URL otherwise.
func (a *App) wizSteps() []string {
	if !a.st.Wiz.mcp {
		return []string{"name", "base url", "api key", "run"}
	}
	if a.st.EditMCP.Transport == "stdio" {
		return []string{"name", "transport", "command", "args"}
	}
	return []string{"name", "transport", "url"}
}

func (a *App) wizFields() []field {
	if a.st.Wiz.mcp {
		return a.mcpFields()
	}
	return a.providerFields()
}

func (a *App) wizField() field {
	steps := a.wizSteps()
	label := steps[min(a.st.Wiz.i, len(steps)-1)]
	for _, f := range a.wizFields() {
		if f.label == label {
			return f
		}
	}
	return field{}
}

// loadStep asks the current field: a text in the composer, with the value it has as
// the default, or a choice that the arrow keys change.
func (a *App) loadStep() {
	f := a.wizField()
	if len(f.opts) > 0 {
		a.endEntry()
		return
	}
	a.beginEntry(entry{label: f.label + " ", secret: f.secret, hint: wizardOptional[f.label], cancel: a.wizardBack, done: func(text string) error {
		if err := f.set(text); err != nil {
			return err
		}
		if err := a.wizardCheck(f.label); err != nil {
			return err
		}
		a.wizardNext()
		return nil
	}}, f.get())
}

// wizardKey handles the keys of a choice step; a text step is an entry.
func (a *App) wizardKey(k *tcell.EventKey) {
	f := a.wizField()
	switch {
	case k.Key() == tcell.KeyEsc:
		a.wizardBack()
	case len(f.opts) == 0:
	case k.Key() == tcell.KeyLeft:
		a.cycleField(f, -1)
	case k.Key() == tcell.KeyRight || k.Rune() == ' ':
		a.cycleField(f, 1)
	case k.Key() == tcell.KeyEnter:
		a.wizardNext()
	}
}

// wizardCheck stops a step that cannot lead to a saved item.
func (a *App) wizardCheck(label string) error {
	if label != "name" {
		return nil
	}
	if a.st.Wiz.mcp {
		s := a.st.EditMCP
		if s.Name == "" {
			return errors.New("name is required")
		}
		for _, o := range a.st.Servers {
			if o.Name == s.Name {
				return errors.New("mcp server " + s.Name + " already exists")
			}
		}
		return nil
	}
	p := a.st.EditProv
	if p.Name == "" {
		return errors.New("name is required")
	}
	if a.providerNamed(p.Name) >= 0 {
		return errors.New("a provider named " + p.Name + " exists")
	}
	return nil
}

func (a *App) wizardNext() {
	a.st.Wiz.i++
	if a.st.Wiz.i < len(a.wizSteps()) {
		a.loadStep()
		return
	}
	if a.st.Wiz.mcp {
		a.finishMCPWizard()
	} else {
		a.finishProviderWizard()
	}
}

// wizardBack drops the new item and returns to the list it was added from.
func (a *App) wizardBack() {
	if a.st.Wiz.mcp {
		a.openOverlay(OverMCP)
		a.st.Sel = len(a.st.Servers)
		return
	}
	a.openOverlay(OverLogin)
	a.st.Sel = firstAddRow(a.loginRows())
}

// finishProviderWizard saves the provider; if it is not valid yet the edit
// form opens and says what is missing.
func (a *App) finishProviderWizard() {
	wasLocked := a.st.Locked
	a.saveProviderEdit()
	if note := a.st.FormNote; note != "" {
		a.openProviderEdit(a.st.EditProv, -1)
		a.st.FormNote = note
		return
	}
	a.openOverlay(OverLogin)
	a.st.Sel = a.loginRowOfProvider(a.st.EditIdx)
	if wasLocked && !a.st.Locked {
		a.closeOverlay()
	}
}

func (a *App) finishMCPWizard() {
	a.saveMCPEdit()
	if note := a.st.FormNote; note != "" {
		a.openOverlay(OverMCPEdit)
		a.st.FormNote = note
		return
	}
	a.openOverlay(OverMCP)
	a.st.Sel = max(a.st.EditIdx, 0)
}
