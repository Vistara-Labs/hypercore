package main

import (
	"fmt"
	"os"
	"vistara-node/internal/command"
)

func main() {
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
