// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version/build info",
		Long:  "Print version/build information",
		Run: func(*cobra.Command, []string) {
			printVersion()
		},
	}
}

func printVersion() {
	fmt.Printf("j9s version: %s\n", displayVersion(version, commit))
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Date: %s\n", date)
}

// displayVersion produces the version string shown in the header. For
// development builds (version == "dev") it appends the short commit SHA so
// it is obvious which build is actually running, e.g. "dev-8989fd5".
// Released builds (version != "dev") are returned as-is.
func displayVersion(version, commit string) string {
	if version == "dev" && commit != "" && commit != "dev" {
		return version + "-" + commit
	}
	return version
}
