package configsvc

import (
	"context"
	"net/url"
	"os"
	"strings"
	"time"

	"meno/internal/api"
	"meno/internal/cliproxy"
	"meno/internal/llm"
)

func (s *Service) loginResource(ctx context.Context, method, path, rawQuery string, req api.ResourceRequest) (any, error) {
	parts := splitResource(strings.TrimPrefix(path, "/login"))
	if len(parts) == 0 || len(parts) == 1 && parts[0] == "services" {
		if method != "GET" {
			return nil, notFound("unsupported login route")
		}
		out := make([]api.ConfigLoginService, 0, len(cliproxy.Services))
		for _, svc := range cliproxy.Services {
			out = append(out, api.ConfigLoginService{ID: svc.ID, Label: svc.Label})
		}
		return api.LoginServicesResponse{Services: out}, nil
	}
	if parts[0] == "accounts" {
		if len(parts) == 1 {
			if method != "GET" {
				return nil, apiErr(api.CodeNotFound, "unsupported account route")
			}
			var accounts []cliproxy.Account
			err := s.withHelper(ctx, func() error { var e error; accounts, e = s.oauth.Accounts(ctx); return e })
			if err != nil {
				return nil, err
			}
			out := make([]api.OAuthAccount, 0, len(accounts))
			for _, a := range accounts {
				out = append(out, accountWire(a))
			}
			return api.AccountsResponse{Accounts: out}, nil
		}
		name, err := url.PathUnescape(parts[1])
		if err != nil {
			return nil, bad(err)
		}
		if len(parts) != 2 {
			return nil, notFound("unsupported account route")
		}
		if method == "DELETE" {
			if err := s.withHelper(ctx, func() error { return s.oauth.Logout(ctx, name) }); err != nil {
				return nil, err
			}
			if err := s.refreshAccountState(ctx); err != nil {
				return nil, err
			}
			s.changed("providers_changed")
			return api.AckResponse{OK: true}, nil
		}
		if method == "PUT" || method == "PATCH" {
			r, err := decode[api.AccountEnabledRequest](req.Data)
			if err != nil {
				return nil, bad(err)
			}
			if err = s.withHelper(ctx, func() error { return s.oauth.SetDisabled(ctx, name, !r.Enabled) }); err != nil {
				return nil, err
			}
			if err = s.refreshAccountState(ctx); err != nil {
				return nil, err
			}
			s.changed("providers_changed")
			return api.AckResponse{OK: true}, nil
		}
		return nil, notFound("unsupported account route")
	}
	switch parts[0] {
	case "helper":
		if method != "GET" {
			return nil, notFound("unsupported helper route")
		}
		st := s.oauth.State()
		return api.OAuthHelperResponse{Installed: s.oauth.Installed(), Running: s.daemons.Running(llm.OAuthName), Ready: s.oauth.Ready(ctx), Version: st.Version, Pending: st.Pending, Port: st.Port, Accounts: st.Accounts, HasAccounts: st.HasAccounts}, nil
	case "start":
		if method != "POST" {
			return nil, notFound("unsupported login route")
		}
		r, err := decode[api.LoginStartRequest](req.Data)
		if err != nil {
			return nil, bad(err)
		}
		if _, ok := cliproxy.ServiceByID(r.Service); !ok {
			return nil, badString("unknown OAuth service " + r.Service)
		}
		var login cliproxy.Login
		loginErr := s.withHelper(ctx, func() error { var e error; login, e = s.oauth.StartLogin(ctx, r.Service); return e })
		if loginErr != nil {
			return nil, loginErr
		}
		return api.LoginStartResponse{URL: login.URL, State: login.State, UserCode: login.UserCode, Device: login.Device, ExpiresIn: int64(login.ExpiresIn.Seconds())}, nil
	case "status":
		if method != "GET" {
			return nil, notFound("unsupported login route")
		}
		state := queryValues(rawQuery).Get("state")
		if state == "" {
			return nil, badString("state is required")
		}
		var status, msg string
		statusErr := s.withHelper(ctx, func() error { var e error; status, msg, e = s.oauth.LoginStatus(ctx, state); return e })
		if statusErr != nil {
			return nil, statusErr
		}
		if status == "ok" {
			if err := s.refreshAccountState(ctx); err != nil {
				return nil, err
			}
		}
		return api.LoginStatusResponse{Status: status, Error: msg}, nil
	case "callback":
		if method != "POST" {
			return nil, notFound("unsupported login route")
		}
		r, err := decode[api.LoginCallbackRequest](req.Data)
		if err != nil {
			return nil, bad(err)
		}
		if err = s.withHelper(ctx, func() error { return s.oauth.SubmitCallback(ctx, r.RedirectURL) }); err != nil {
			return nil, err
		}
		return api.AckResponse{OK: true}, nil
	case "restart":
		if method != "POST" {
			return nil, notFound("unsupported login route")
		}
		if err := s.restartHelper(ctx); err != nil {
			return nil, err
		}
		return api.AckResponse{OK: true}, nil
	}
	return nil, notFound("unsupported login route")
}

func (s *Service) ensureHelper(ctx context.Context) error {
	return s.withHelper(ctx, func() error { return nil })
}
func (s *Service) restartHelper(ctx context.Context) error {
	if err := s.oauth.Ensure(ctx, nil); err != nil {
		return err
	}
	s.daemons.Stop(llm.OAuthName)
	if !s.oauth.PortFree() {
		s.oauth.ChoosePort()
	}
	if err := s.oauth.WriteConfig(); err != nil {
		return err
	}
	if err := s.daemons.Start(llm.OAuthName, s.oauth.Command(), s.paths.Home, os.Environ()); err != nil {
		return err
	}
	wait, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.oauth.WaitReady(wait)
}
func (s *Service) refreshAccountState(ctx context.Context) error {
	var accounts []cliproxy.Account
	if err := s.withHelper(ctx, func() error { var e error; accounts, e = s.oauth.Accounts(ctx); return e }); err != nil {
		return err
	}
	enabled := 0
	for _, a := range accounts {
		if !a.Disabled {
			enabled++
		}
	}
	s.oauth.Update(func(st *cliproxy.State) {
		st.Accounts = len(accounts)
		st.AccountsKnown = true
		st.HasAccounts = enabled > 0
	})
	if len(accounts) == 0 {
		s.daemons.Stop(llm.OAuthName)
	}
	s.changed("providers_changed")
	return nil
}
func accountWire(a cliproxy.Account) api.OAuthAccount {
	return api.OAuthAccount{File: a.File, Provider: a.Provider, Service: a.Service, Label: a.Label, Plan: a.Plan, Disabled: a.Disabled, Failed: a.Failed != 0, Note: a.Note}
}
func badString(message string) error { return apiErr(api.CodeBadRequest, message) }
