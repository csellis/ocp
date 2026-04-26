// Command ocp is the OCP command-line entry point.
//
// v0.1 subcommands (scan, drift, respond) are added incrementally.
// See docs/PLAN.md for the build order. This stub exists so the
// Go toolchain has at least one buildable package.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	fmt.Fprintln(os.Stderr, "ocp: subcommands not yet implemented; see docs/PLAN.md")
	os.Exit(1)
}
