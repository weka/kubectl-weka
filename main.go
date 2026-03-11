package main

import "github.com/weka/kubectl-weka/cmd"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Set version information in cmd package before executing
	cmd.SetVersion(version, commit, date)
	cmd.Execute()
}
