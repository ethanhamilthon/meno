// Package configsvc implements daemon-owned configuration resources.
package configsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/process"
)

// Options injects daemon-owned configuration and process managers.
type Options struct {
	Paths         config.Paths
	Store         *config.Store
	Daemons       *process.Daemons
	MCP           *mcp.Manager
	Notify        func(event string)
	DebugRequests func(session string) []json.RawMessage
	FetchModels   func(context.Context, llm.Provider) ([]llm.Model, error)
	FetchMetadata func(context.Context) (map[string]llm.ModelInfo, error)
}

// Service owns configuration reads, writes, and helper lifecycles.
type Service struct {
	paths          config.Paths
	store          *config.Store
	daemons        *process.Daemons
	mcp            *mcp.Manager
	notify         func(string)
	debug          func(string) []json.RawMessage
	fetchModels    func(context.Context, llm.Provider) ([]llm.Model, error)
	fetchMetadata  func(context.Context) (map[string]llm.ModelInfo, error)
	mu             sync.Mutex
	settingsMu     sync.Mutex
	mcpMu          sync.Mutex
	files          map[string]fileStamp
	settings       config.Settings
	settingsLoaded bool
	bg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	knownModels    map[string][]llm.Model
	modelErrors    map[string]string
}

type fileStamp struct {
	mod    int64
	size   int64
	exists bool
}

// New creates a service. It uses the MENO_HOME path when Paths.Home is empty.
func New(o Options) (*Service, error) {
	if o.Paths.Home == "" {
		p, err := config.DefaultPaths()
		if err != nil {
			return nil, err
		}
		o.Paths = p
	}
	if o.Paths.Cwd == "" {
		o.Paths.Cwd = "."
	}
	if o.Store == nil {
		o.Store = config.NewStore(o.Paths)
	}
	if o.Daemons == nil {
		o.Daemons = process.NewDaemons(o.Paths.File("providers"))
	}
	if o.MCP == nil {
		o.MCP = mcp.NewManager()
	}
	svc := &Service{paths: o.Paths, store: o.Store, daemons: o.Daemons, mcp: o.MCP,
		notify: o.Notify, debug: o.DebugRequests, fetchModels: o.FetchModels, fetchMetadata: o.FetchMetadata,
		files: map[string]fileStamp{}, knownModels: map[string][]llm.Model{}, modelErrors: map[string]string{}}
	svc.mcp.SetOnChange(func(string) { svc.changed("mcp_changed") })
	return svc, nil
}

// Resource dispatches a daemon resource. Paths include the /v1 prefix.
func (s *Service) Resource(ctx context.Context, method, path, query string, req api.ResourceRequest) (api.ResourceResponse, error) {
	if !strings.HasPrefix(path, "/v1/") {
		return api.ResourceResponse{}, apiErr(api.CodeNotFound, "resource not found")
	}
	path = strings.TrimPrefix(path, "/v1")
	var data any
	var err error
	switch {
	case path == "/config":
		data, err = s.bundleResource(method)
	case path == "/settings":
		data, err = s.settingsResource(method, req)
	case path == "/providers" || strings.HasPrefix(path, "/providers/"):
		data, err = s.providerResource(ctx, method, path, query, req)
	case path == "/skills" || strings.HasPrefix(path, "/skills/"):
		data, err = s.skillsResource(method, path, req)
	case path == "/mcp/servers" || strings.HasPrefix(path, "/mcp/servers/"):
		data, err = s.mcpResource(ctx, method, path, req)
	case path == "/models":
		data, err = s.modelsResource(ctx, method, query)
	case path == "/fs/search":
		data, err = s.searchResource(ctx, query)
	case path == "/debug/requests":
		data, err = s.debugResource(query)
	default:
		return api.ResourceResponse{}, apiErr(api.CodeNotFound, "resource not found")
	}
	if err != nil {
		return api.ResourceResponse{}, err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return api.ResourceResponse{}, err
	}
	return api.ResourceResponse{Data: b}, nil
}

func (s *Service) SetNotify(notify func(string)) { s.mu.Lock(); s.notify = notify; s.mu.Unlock() }

func (s *Service) changed(event string) {
	s.mu.Lock()
	notify := s.notify
	s.mu.Unlock()
	if notify != nil {
		notify(event)
	}
}
func apiErr(code, message string) error { return &daemon.APIError{Code: code, Message: message} }
func badString(message string) error    { return apiErr(api.CodeBadRequest, message) }
func bad(err error) error               { return apiErr(api.CodeBadRequest, err.Error()) }
func notFound(message string) error     { return apiErr(api.CodeNotFound, message) }
func conflict(message string) error     { return apiErr(api.CodeConflict, message) }
func decode[T any](raw json.RawMessage) (T, error) {
	var value T
	if len(raw) == 0 {
		return value, errors.New("request data is required")
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, err
	}
	return value, nil
}
func splitResource(path string) []string {
	part := strings.Trim(path, "/")
	if part == "" {
		return nil
	}
	return strings.Split(part, "/")
}
func queryValues(raw string) url.Values { v, _ := url.ParseQuery(raw); return v }
func queryInt(q url.Values, key string, fallback int) int {
	n, err := strconv.Atoi(q.Get(key))
	if err != nil {
		return fallback
	}
	return n
}
func (s *Service) stamp(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{mod: info.ModTime().UnixNano(), size: info.Size(), exists: true}
}
func (s *Service) updateStamp(path string, stamp fileStamp) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.files[path]
	s.files[path] = stamp
	return ok && old != stamp
}
func checkMethod(method string, allowed ...string) error {
	for _, m := range allowed {
		if method == m {
			return nil
		}
	}
	return apiErr(api.CodeNotFound, fmt.Sprintf("method %s is not supported", method))
}
