package main

import (
	"os"

	"github.com/ManuelLuchage/apura/internal/cli"
)

// version is set at build time: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
