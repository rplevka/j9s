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
	fmt.Printf("j9s version: %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Date: %s\n", date)
}
