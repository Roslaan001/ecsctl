package main

import (
	"os"

	"github.com/roslaan001/ecsctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
