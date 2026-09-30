package main

import (
	"github.com/TTstack/tt/internal/agent"
	"os"
)

func main() { os.Exit(agent.RunUpgrade(os.Args[1:], os.Stdout, os.Stderr)) }
