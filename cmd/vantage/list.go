package main

import (
	"fmt"
	"io"

	"github.com/disun001/vantage/internal/workspace"
)

var listCmd = command{
	name:    "list",
	usage:   "list",
	summary: "list saved workspaces",
	run:     runList,
}

func runList(_ []string, out io.Writer) error {
	names, err := workspace.List()
	if err != nil {
		return err
	}
	for _, n := range names {
		fmt.Fprintln(out, n)
	}
	return nil
}
