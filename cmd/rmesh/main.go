package main

import (
	"os"

	"github.com/relaymonkey/relaymesh-edge/cmd/rmesh/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
