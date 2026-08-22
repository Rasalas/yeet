package main

import "github.com/rasalas/yeet/cmd"

// version is injected at build time:
//
//	go build -ldflags "-X main.version=v1.2.3" .
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
