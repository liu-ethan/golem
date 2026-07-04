package main

import "os"

// version 在构建时可通过 -ldflags 注入。
var version = "v0.1.0-dev"

func main() {
	os.Exit(mainEntry())
}
