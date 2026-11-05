package main

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	pb "termium/client/pb"
)

// Ghostty, Kitty and WezTerm negotiate the kitty keyboard protocol, so keys
// arrive as CSI u. They must keep the meaning legacy input had.
func TestKittyKeyboardKeepsLegacyKeyMeaning(t *testing.T) {
	t.Setenv("TCELL_KEYBOARD_PROTOCOL", "kitty")
	s := uiScreen(t, 80, 24)
	if s.KeyboardProtocol() != tcell.KittyKeyboard {
		t.Fatalf("keyboard protocol %v, want kitty", s.KeyboardProtocol())
	}
	read := func(raw string) *tcell.EventKey {
		t.Helper()
		s.term.SendRaw([]byte(raw))
		deadline := time.After(2 * time.Second)
		for {
			select {
			case ev := <-s.EventQ():
				if k, ok := ev.(*tcell.EventKey); ok {
					return k
				}
			case <-deadline:
				t.Fatalf("no key event for %q", raw)
			}
		}
	}
	for raw, want := range map[string]tcell.Key{
		"\x1b[9;2u": tcell.KeyBacktab, "\x1b[27u": tcell.KeyEscape,
		"\x1b[13u": tcell.KeyEnter, "\x1b[127u": tcell.KeyBackspace,
	} {
		if got := read(raw).Key(); got != want {
			t.Errorf("%q decoded as %v, want %v", raw, got, want)
		}
	}
	for raw, want := range map[string]struct {
		key tcell.Key
		r   rune
	}{
		"\x1b[108;5u": {tcell.KeyCtrlL, 'l'}, "\x1b[108;6u": {tcell.KeyCtrlL, 'l'},
		"\x1b[113;5u": {tcell.KeyCtrlQ, 'q'}, "\x1b[116;5u": {tcell.KeyCtrlT, 't'},
		"\x1b[119;5u": {tcell.KeyCtrlW, 'w'},
	} {
		if ev := read(raw); !ctrl(ev, want.key, want.r) {
			t.Errorf("%q (%s) is not Ctrl+%c", raw, ev.Name(), want.r)
		}
	}
	kh, ops := recorder()
	kh.focus = "page"
	kh.pageKey(read("\x1b[120;3u"))
	if len(*ops) != 1 || (*ops)[0].input.Kind != pb.InputKind_KEY_INPUT || (*ops)[0].input.Key != "x" || (*ops)[0].input.Modifiers != 1 {
		t.Fatalf("Alt+x reached the page as %+v", *ops)
	}
}
