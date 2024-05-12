package app

import (
	"vistara-node/pkg/ports"
)

// App is the interface for the core vistara app.
type App interface {
	ports.MicroVMService
}

func New(cfg *Config, ports *ports.Collection) App {
	return &app{
		cfg:   cfg,
		ports: ports,
	}
}

type app struct {
	cfg   *Config
	ports *ports.Collection
}

type Config struct {
	RootStateDir    string
	MaximumRetry    int
	DefaultProvider string
}
