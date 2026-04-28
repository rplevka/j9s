// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
)

// ResourceViewer represents a resource viewer interface.
type ResourceViewer interface {
	model.Component
	tview.Primitive
}

// IDProvider is implemented by views that expose the first-column IDs of
// the rows they currently display. Used by command-prompt argument
// autocomplete to suggest values from whatever the user is looking at.
type IDProvider interface {
	IDs() []string
}

// PathProvider is implemented by views that live inside a folder
// hierarchy. The command-prompt autocomplete uses the returned path to
// qualify suggested IDs so accepted suggestions navigate to the right
// nested location instead of the root.
type PathProvider interface {
	CurrentPath() string
}
