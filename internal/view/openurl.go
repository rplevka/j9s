// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openURLFunc is the function used to open a URL in the system's default
// browser. It is a package-level var so tests can swap in a recorder
// without spawning real processes. Returning a wrapped error preserves
// the underlying exec error for diagnostic flashes.
var openURLFunc = openURL

// openURL launches the system browser at the given URL. Supports macOS
// (`open`), Linux (`xdg-open`) and Windows (`rundll32 url.dll,FileProtocolHandler`).
// Reports the platform-specific error to the caller so it can be surfaced
// via a flash.
func openURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty URL")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		// rundll32 sidesteps cmd.exe quoting issues with URLs containing &.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("don't know how to open a browser on %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}
	// Don't Wait() — `open` and `xdg-open` typically fork the actual
	// browser and return immediately, but on some setups they linger.
	// Releasing means the j9s process isn't blocked behind the browser.
	go func() { _ = cmd.Wait() }()
	return nil
}
