package configsvc

import (
	"os"

	"meno/internal/llm"
)

func (s *Service) syncProvider(p llm.Provider) {
	if p.Disabled {
		s.daemons.Stop(p.Name)
		return
	}
	if p.Managed() {
		_ = s.oauth.WriteConfig()
	}
	_ = s.daemons.Start(p.Name, p.Run, s.paths.Cwd, os.Environ())
}
