package configsvc

import (
	"context"
	"errors"
	"time"

	"meno/internal/cliproxy"
	"meno/internal/llm"
)

// Start starts enabled providers and MCP servers. It also starts an installed
// OAuth helper when account state says that accounts may exist.
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
	oldExit := s.daemons.OnExit
	s.daemons.OnExit = func(name string, err error, log string) {
		if oldExit != nil {
			oldExit(name, err, log)
		}
		if name == llm.OAuthName && s.oauth.State().Accounts > 0 {
			go s.recoverHelper(runCtx)
		}
	}
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
		if p.Active() && !p.Managed() {
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
	st := s.oauth.State()
	if s.oauth.Installed() && (st.Accounts > 0 || !st.AccountsKnown) {
		go s.startHelper(runCtx)
	}
	return nil
}

// Close stops the helper, MCP clients, and supervised provider processes.
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

func (s *Service) startHelper(ctx context.Context) {
	if err := s.ensureHelper(ctx); err != nil || ctx.Err() != nil {
		return
	}
	_ = s.refreshAccountState(ctx)
}
func (s *Service) recoverHelper(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		if s.oauth.State().Accounts == 0 {
			return
		}
		if err := s.withHelper(ctx, func() error { return nil }); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}
func (s *Service) withHelper(ctx context.Context, fn func() error) error {
	s.helperMu.Lock()
	defer s.helperMu.Unlock()
	for try := 0; try < 2; try++ {
		if !s.oauth.Ready(ctx) {
			if err := s.oauth.Ensure(ctx, nil); err != nil {
				return err
			}
			if err := s.restartHelper(ctx); err != nil {
				return err
			}
		}
		err := fn()
		if !errors.Is(err, cliproxy.ErrHelperDown) {
			return err
		}
		if try == 1 {
			return err
		}
		if err = s.restartHelper(ctx); err != nil {
			return err
		}
	}
	return nil
}
