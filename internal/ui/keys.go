// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import "github.com/derailed/tcell/v2"

// Key constants for common keys.
const (
	KeyColon    tcell.Key = tcell.Key(':')
	KeySlash    tcell.Key = tcell.Key('/')
	KeySpace    tcell.Key = tcell.Key(' ')
	KeyQuestion tcell.Key = tcell.Key('?')

	KeyA tcell.Key = tcell.Key('a')
	KeyB tcell.Key = tcell.Key('b')
	KeyC tcell.Key = tcell.Key('c')
	KeyD tcell.Key = tcell.Key('d')
	KeyE tcell.Key = tcell.Key('e')
	KeyF tcell.Key = tcell.Key('f')
	KeyG tcell.Key = tcell.Key('g')
	KeyH tcell.Key = tcell.Key('h')
	KeyI tcell.Key = tcell.Key('i')
	KeyJ tcell.Key = tcell.Key('j')
	KeyK tcell.Key = tcell.Key('k')
	KeyL tcell.Key = tcell.Key('l')
	KeyM tcell.Key = tcell.Key('m')
	KeyN tcell.Key = tcell.Key('n')
	KeyO tcell.Key = tcell.Key('o')
	KeyP tcell.Key = tcell.Key('p')
	KeyQ tcell.Key = tcell.Key('q')
	KeyR tcell.Key = tcell.Key('r')
	KeyS tcell.Key = tcell.Key('s')
	KeyT tcell.Key = tcell.Key('t')
	KeyU tcell.Key = tcell.Key('u')
	KeyV tcell.Key = tcell.Key('v')
	KeyW tcell.Key = tcell.Key('w')
	KeyX tcell.Key = tcell.Key('x')
	KeyY tcell.Key = tcell.Key('y')
	KeyZ tcell.Key = tcell.Key('z')

	KeyShiftA tcell.Key = tcell.Key('A')
	KeyShiftB tcell.Key = tcell.Key('B')
	KeyShiftC tcell.Key = tcell.Key('C')
	KeyShiftD tcell.Key = tcell.Key('D')
	KeyShiftE tcell.Key = tcell.Key('E')
	KeyShiftF tcell.Key = tcell.Key('F')
	KeyShiftG tcell.Key = tcell.Key('G')
	KeyShiftK tcell.Key = tcell.Key('K')
	KeyShiftL tcell.Key = tcell.Key('L')
	KeyShiftN tcell.Key = tcell.Key('N')
	KeyShiftR tcell.Key = tcell.Key('R')
	KeyShiftS tcell.Key = tcell.Key('S')

	Key0 tcell.Key = tcell.Key('0')
	Key1 tcell.Key = tcell.Key('1')
	Key2 tcell.Key = tcell.Key('2')
	Key3 tcell.Key = tcell.Key('3')
	Key4 tcell.Key = tcell.Key('4')
	Key5 tcell.Key = tcell.Key('5')
	Key6 tcell.Key = tcell.Key('6')
	Key7 tcell.Key = tcell.Key('7')
	Key8 tcell.Key = tcell.Key('8')
	Key9 tcell.Key = tcell.Key('9')
)

// initShiftKeyNames registers human-readable names for Shift-letter keys so
// they render distinctly in the menu (e.g. "<shift-d>" vs "<d>"). Mirrors the
// k9s pattern (see internal/ui/key.go in k9s).
func initShiftKeyNames() {
	tcell.KeyNames[KeyShiftA] = "Shift-A"
	tcell.KeyNames[KeyShiftB] = "Shift-B"
	tcell.KeyNames[KeyShiftC] = "Shift-C"
	tcell.KeyNames[KeyShiftD] = "Shift-D"
	tcell.KeyNames[KeyShiftE] = "Shift-E"
	tcell.KeyNames[KeyShiftF] = "Shift-F"
	tcell.KeyNames[KeyShiftG] = "Shift-G"
	tcell.KeyNames[KeyShiftK] = "Shift-K"
	tcell.KeyNames[KeyShiftL] = "Shift-L"
	tcell.KeyNames[KeyShiftN] = "Shift-N"
	tcell.KeyNames[KeyShiftR] = "Shift-R"
	tcell.KeyNames[KeyShiftS] = "Shift-S"
}

func init() {
	initShiftKeyNames()
}
