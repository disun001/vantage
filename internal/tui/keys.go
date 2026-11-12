package tui

import tea "github.com/charmbracelet/bubbletea"

// keyBytes encodes a Bubble Tea key as the bytes a terminal would send, so
// it can be forwarded to the embedded tmux client. ok is false for keys with
// no encoding (the caller drops them).
func keyBytes(k tea.KeyMsg) (b []byte, ok bool) {
	if k.Type == tea.KeyRunes {
		b = []byte(string(k.Runes))
	} else if seq, found := specialKeys[k.Type]; found {
		b = []byte(seq)
	} else {
		return nil, false
	}
	if k.Alt {
		b = append([]byte{0x1b}, b...)
	}
	return b, true
}

// specialKeys uses the xterm sequences tmux expects with TERM=xterm-256color.
var specialKeys = map[tea.KeyType]string{
	tea.KeyEnter:      "\r",
	tea.KeyTab:        "\t",
	tea.KeyBackspace:  "\x7f",
	tea.KeyEscape:     "\x1b",
	tea.KeySpace:      " ",
	tea.KeyUp:         "\x1b[A",
	tea.KeyDown:       "\x1b[B",
	tea.KeyRight:      "\x1b[C",
	tea.KeyLeft:       "\x1b[D",
	tea.KeyHome:       "\x1b[H",
	tea.KeyEnd:        "\x1b[F",
	tea.KeyPgUp:       "\x1b[5~",
	tea.KeyPgDown:     "\x1b[6~",
	tea.KeyDelete:     "\x1b[3~",
	tea.KeyInsert:     "\x1b[2~",
	tea.KeyShiftTab:   "\x1b[Z",
	tea.KeyF1:         "\x1bOP",
	tea.KeyF2:         "\x1bOQ",
	tea.KeyF3:         "\x1bOR",
	tea.KeyF4:         "\x1bOS",
	tea.KeyF5:         "\x1b[15~",
	tea.KeyF6:         "\x1b[17~",
	tea.KeyF7:         "\x1b[18~",
	tea.KeyF8:         "\x1b[19~",
	tea.KeyF9:         "\x1b[20~",
	tea.KeyF10:        "\x1b[21~",
	tea.KeyF11:        "\x1b[23~",
	tea.KeyF12:        "\x1b[24~",
	tea.KeyCtrlAt:     "\x00",
	tea.KeyCtrlH:      "\x08",
	tea.KeyCtrlJ:      "\x0a",
	tea.KeyCtrlUp:     "\x1b[1;5A",
	tea.KeyCtrlDown:   "\x1b[1;5B",
	tea.KeyCtrlRight:  "\x1b[1;5C",
	tea.KeyCtrlLeft:   "\x1b[1;5D",
	tea.KeyShiftUp:    "\x1b[1;2A",
	tea.KeyShiftDown:  "\x1b[1;2B",
	tea.KeyShiftRight: "\x1b[1;2C",
	tea.KeyShiftLeft:  "\x1b[1;2D",
	tea.KeyCtrlA:      "\x01", tea.KeyCtrlB: "\x02", tea.KeyCtrlC: "\x03", tea.KeyCtrlD: "\x04",
	tea.KeyCtrlE: "\x05", tea.KeyCtrlF: "\x06", tea.KeyCtrlG: "\x07", tea.KeyCtrlK: "\x0b",
	tea.KeyCtrlL: "\x0c", tea.KeyCtrlN: "\x0e", tea.KeyCtrlO: "\x0f", tea.KeyCtrlP: "\x10",
	tea.KeyCtrlQ: "\x11", tea.KeyCtrlR: "\x12", tea.KeyCtrlS: "\x13", tea.KeyCtrlT: "\x14",
	tea.KeyCtrlU: "\x15", tea.KeyCtrlV: "\x16", tea.KeyCtrlW: "\x17", tea.KeyCtrlX: "\x18",
	tea.KeyCtrlY: "\x19", tea.KeyCtrlZ: "\x1a", tea.KeyCtrlBackslash: "\x1c",
	tea.KeyCtrlCloseBracket: "\x1d", tea.KeyCtrlCaret: "\x1e", tea.KeyCtrlUnderscore: "\x1f",
}
