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
