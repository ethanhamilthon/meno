package configsvc

import (
	"context"
	"time"

	"meno/internal/llm"
)

// Start starts enabled providers and MCP servers.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.ctx = runCtx
	s.cancel = cancel
	s.mu.Unlock()
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		s.refreshMetadata(runCtx)
	}()
	providers, err := s.providers()
	if err != nil {
		cancel()
		return err
	}
	for _, p := range providers {
		if p.Active() {
			s.syncProvider(p)
			prov := p
			s.bg.Add(1)
			go func() {
				defer s.bg.Done()
				s.refreshModels(runCtx, prov)
			}()
		}
	}
	servers, err := s.servers()
	if err != nil {
		cancel()
		return err
	}
	for _, x := range servers {
		if x.Enabled {
			server := x
			s.mcp.StartAsync(runCtx, server)
		}
	}
	return nil
}

// Close stops MCP clients and supervised provider processes.
func (s *Service) Close() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.daemons.StopAll()
	s.mcp.StopAll()
	s.bg.Wait()
}

func (s *Service) refreshMetadata(ctx context.Context) {
	fetch := s.fetchMetadata
	if fetch == nil {
		fetch = llm.FetchMetadata
	}
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	catalog, err := fetch(callCtx)
	if err != nil || ctx.Err() != nil {
		return
	}
	_ = s.store.SaveCatalog(catalog)
}

func (s *Service) refreshModels(ctx context.Context, p llm.Provider) {
	fetch := s.fetchModels
	if fetch == nil {
		fetch = llm.FetchModels
	}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	models, err := fetch(callCtx, p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.modelErrors[p.Name] = err.Error()
		return
	}
	s.knownModels[p.Name] = models
	delete(s.modelErrors, p.Name)
}
