// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package config

// Flags represents command line flags.
type Flags struct {
	RefreshRate *float32
	LogLevel    *string
	LogFile     *string
	Headless    *bool
	Logoless    *bool
	Command     *string
	ReadOnly    *bool
	Context     *string
}

// NewFlags returns a new flags instance with defaults.
func NewFlags() *Flags {
	refreshRate := float32(DefaultRefreshRate)
	logLevel := DefaultLogLevel
	logFile := AppLogFile
	headless := false
	logoless := false
	command := DefaultCommand
	readOnly := false
	context := ""

	return &Flags{
		RefreshRate: &refreshRate,
		LogLevel:    &logLevel,
		LogFile:     &logFile,
		Headless:    &headless,
		Logoless:    &logoless,
		Command:     &command,
		ReadOnly:    &readOnly,
		Context:     &context,
	}
}
