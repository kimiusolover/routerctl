package main

import (
	"fmt"
	"os"

	"github.com/example/routerctl/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.Run(os.Args[1:], cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "routerctl:", err)
		os.Exit(1)
	}
}
