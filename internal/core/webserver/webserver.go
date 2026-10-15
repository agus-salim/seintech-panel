package webserver

import (
	"context"
	"fmt"

	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
)

// WebServer defines the standard contract implemented by each web server engine.
type WebServer interface {
	Type() model.WebServerType
	Name() string
	GenerateVHost(ctx context.Context, domain *model.Domain) error
	RemoveVHost(ctx context.Context, domain *model.Domain) error
	TestConfig(ctx context.Context) (string, error)
	Reload(ctx context.Context) error
	Restart(ctx context.Context) error
	Status(ctx context.Context) (system.ServiceState, error)
	ConfigPath(domain *model.Domain) string
}

type Registry struct {
	servers map[model.WebServerType]WebServer
}

func NewRegistry(sm *system.ServiceManager) *Registry {
	r := &Registry{
		servers: make(map[model.WebServerType]WebServer),
	}

	r.Register(NewNginxServer(sm))
	r.Register(NewApacheServer(sm))

	return r
}

func (r *Registry) Register(ws WebServer) {
	r.servers[ws.Type()] = ws
}

func (r *Registry) Get(wsType model.WebServerType) (WebServer, error) {
	ws, ok := r.servers[wsType]
	if !ok {
		return nil, fmt.Errorf("unsupported webserver engine: %s", wsType)
	}
	return ws, nil
}

func (r *Registry) All() []WebServer {
	var list []WebServer
	for _, ws := range r.servers {
		list = append(list, ws)
	}
	return list
}
