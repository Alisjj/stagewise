package main

import (
	"os"

	"stagewise/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
