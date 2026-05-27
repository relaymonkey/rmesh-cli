package main

import (
	"os"

	"github.com/relaymonkey/relaymesh-edge/cmd/rmesh/cmd"
)

var version = "0.0.0-dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
