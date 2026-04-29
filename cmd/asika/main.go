package main

import (
	"os"

	"asika/lib/commands"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
