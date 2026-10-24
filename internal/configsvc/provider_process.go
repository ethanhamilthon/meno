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
	_ = s.daemons.Start(p.Name, p.Run, s.paths.Cwd, os.Environ())
}
