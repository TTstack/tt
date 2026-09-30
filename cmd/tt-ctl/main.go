package main

import (
	"github.com/TTstack/tt/internal/ctl"
	"os"
)

func main() { os.Exit(ctl.Run(os.Args[1:], os.Stdout, os.Stderr)) }
