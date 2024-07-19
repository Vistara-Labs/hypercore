package main

import (
	"fmt"
	"os"
	"path/filepath"
	"vistara-node/internal/command"
	"vistara-node/pkg/shim"
)

func main() {
	if filepath.Base(os.Args[0]) == "containerd-shim-hypercore-example" {
		shim.Run()
		return
	}

	rootCmd, err := command.NewRootCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}
