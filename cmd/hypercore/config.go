package main

import "vistara-node/pkg/log"

type Config struct {
	CtrSocketPath     string
	CtrNamespace      string
	DefaultVMProvider string
	HACFile           string
	Logging           log.Config
}
