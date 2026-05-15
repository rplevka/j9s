// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package main

import (
	"os"

	"github.com/rplevka/j9s/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
