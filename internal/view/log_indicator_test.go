// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"strings"
	"testing"
)

func TestLogIndicator_NewLogIndicator(t *testing.T) {
	indicator := NewLogIndicator()

	if indicator == nil {
		t.Fatal("expected indicator to be created")
	}

	// Default values
	if !indicator.autoScroll {
		t.Error("expected autoScroll to be true by default")
	}
	if !indicator.textWrap {
		t.Error("expected textWrap to be true by default")
	}
	if indicator.fullScreen {
		t.Error("expected fullScreen to be false by default")
	}
}

func TestLogIndicator_SetAutoScroll(t *testing.T) {
	indicator := NewLogIndicator()

	indicator.SetAutoScroll(false)
	if indicator.autoScroll {
		t.Error("expected autoScroll to be false")
	}

	text := indicator.GetText(false)
	if !strings.Contains(text, "Autoscroll:") {
		t.Error("expected indicator text to contain Autoscroll")
	}
	if strings.Contains(text, "On") && !strings.Contains(text, "Off") {
		t.Error("expected Autoscroll to show Off")
	}

	indicator.SetAutoScroll(true)
	if !indicator.autoScroll {
		t.Error("expected autoScroll to be true")
	}
}

func TestLogIndicator_SetTextWrap(t *testing.T) {
	indicator := NewLogIndicator()

	indicator.SetTextWrap(false)
	if indicator.textWrap {
		t.Error("expected textWrap to be false")
	}

	indicator.SetTextWrap(true)
	if !indicator.textWrap {
		t.Error("expected textWrap to be true")
	}
}

func TestLogIndicator_SetFullScreen(t *testing.T) {
	indicator := NewLogIndicator()

	indicator.SetFullScreen(true)
	if !indicator.fullScreen {
		t.Error("expected fullScreen to be true")
	}

	indicator.SetFullScreen(false)
	if indicator.fullScreen {
		t.Error("expected fullScreen to be false")
	}
}

func TestLogIndicator_Refresh(t *testing.T) {
	indicator := NewLogIndicator()

	// Test with all on
	indicator.SetAutoScroll(true)
	indicator.SetTextWrap(true)
	indicator.SetFullScreen(true)

	text := indicator.GetText(false)
	if !strings.Contains(text, "Autoscroll:") {
		t.Error("expected indicator to contain Autoscroll")
	}
	if !strings.Contains(text, "FullScreen:") {
		t.Error("expected indicator to contain FullScreen")
	}
	if !strings.Contains(text, "Wrap:") {
		t.Error("expected indicator to contain Wrap")
	}
}
