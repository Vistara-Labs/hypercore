package main

import (
	"os"
	"path/filepath"
	"vistara-node/internal/hypercore"
	"vistara-node/pkg/shim"
)

func main() {
	switch filepath.Base(os.Args[0]) {
	case "containerd-shim-hypercore-example":
		shim.Run()
	default:
		hypercore.Run()
	}
}
