//go:build linux

package midi

import (
	"os"
	"syscall"
	"testing"
	"time"
)

type ev struct{ note, vel, ch int }

func run(t *testing.T, bytes []byte) []ev {
	t.Helper()
	var p parser
	var got []ev
	for _, b := range bytes {
		if n, v, c, ok := p.feed(b); ok {
			got = append(got, ev{n, v, c})
		}
	}
	return got
}

func TestParser(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []ev
	}{
		{
			"plain note on/off",
			[]byte{0x90, 60, 97, 0x80, 60, 0},
			[]ev{{60, 97, 0}},
		},
		{
			"note on with velocity 0 is note off",
			[]byte{0x90, 60, 97, 0x90, 60, 0},
			[]ev{{60, 97, 0}},
		},
		{
			"running status: repeated note ons without status byte",
			[]byte{0x90, 60, 97, 60, 0, 62, 80, 62, 0},
			[]ev{{60, 97, 0}, {62, 80, 0}},
		},
		{
			"active sensing interleaved mid-message must not break running status",
			[]byte{0x90, 0xFE, 60, 0xFE, 97, 0xFE, 60, 0, 0xFE, 62, 80},
			[]ev{{60, 97, 0}, {62, 80, 0}},
		},
		{
			"clock and other real-time bytes are ignored",
			[]byte{0xF8, 0x90, 60, 97, 0xF8, 0xFA, 0xFC},
			[]ev{{60, 97, 0}},
		},
		{
			"sysex payload is skipped, note after it still parses",
			[]byte{0xF0, 0x7E, 0x00, 0x06, 0x01, 0xF7, 0x90, 60, 97},
			[]ev{{60, 97, 0}},
		},
		{
			"sysex containing note-like data bytes emits nothing",
			[]byte{0xF0, 60, 97, 62, 80, 0xF7},
			nil,
		},
		{
			"control change (2 data bytes) does not trigger",
			[]byte{0xB0, 7, 100, 0x90, 60, 97},
			[]ev{{60, 97, 0}},
		},
		{
			"program change has 1 data byte; a following note still parses",
			[]byte{0xC0, 5, 0x90, 60, 97},
			[]ev{{60, 97, 0}},
		},
		{
			"channel pressure has 1 data byte",
			[]byte{0xD0, 64, 0x90, 60, 97},
			[]ev{{60, 97, 0}},
		},
		{
			"pitch bend has 2 data bytes",
			[]byte{0xE0, 0, 64, 0x90, 60, 97},
			[]ev{{60, 97, 0}},
		},
		{
			"song position pointer (F2, 2 data bytes) cancels running status",
			[]byte{0x90, 60, 97, 0xF2, 0x00, 0x10, 62, 80, 0x90, 64, 90},
			[]ev{{60, 97, 0}, {64, 90, 0}},
		},
		{
			"channel is decoded from the low nibble",
			[]byte{0x9A, 60, 97},
			[]ev{{60, 97, 10}},
		},
		{
			"stream joined mid-message: leading data bytes ignored",
			[]byte{60, 97, 0x90, 62, 80},
			[]ev{{62, 80, 0}},
		},
		{
			"tune request (F6, 0 data bytes) then a note",
			[]byte{0xF6, 0x90, 60, 97},
			[]ev{{60, 97, 0}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("event %d: got %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestListenEndToEnd drives the full listener over a FIFO, exercising
// open -> parse -> note/channel filter -> debounce -> callback.
func TestListenEndToEnd(t *testing.T) {
	fifo := t.TempDir() + "/midi.fifo"
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}

	triggers := make(chan struct{}, 32)
	cfg := Config{Device: fifo, Note: 60, LangNote: noLangNote, Channel: anyChannel, Debounce: 250 * time.Millisecond}
	go Run(cfg, func(note int) { triggers <- struct{}{} })

	w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	send := func(b ...byte) {
		if _, err := w.Write(b); err != nil {
			t.Fatal(err)
		}
	}

	count := func(within time.Duration) int {
		n := 0
		deadline := time.After(within)
		for {
			select {
			case <-triggers:
				n++
			case <-deadline:
				return n
			}
		}
	}

	// Active sensing spam alone must never trigger.
	for i := 0; i < 20; i++ {
		send(0xFE)
	}
	if n := count(150 * time.Millisecond); n != 0 {
		t.Fatalf("active sensing triggered %d times, want 0", n)
	}

	// One press of note 60 = exactly one toggle (note-off must not add one).
	send(0x90, 60, 97)
	send(0x80, 60, 0)
	if n := count(150 * time.Millisecond); n != 1 {
		t.Fatalf("single press gave %d triggers, want 1", n)
	}

	// The wrong note must be ignored.
	send(0x90, 62, 97, 0x80, 62, 0)
	if n := count(150 * time.Millisecond); n != 0 {
		t.Fatalf("wrong note gave %d triggers, want 0", n)
	}

	// Rapid repeats inside the debounce window collapse to one.
	time.Sleep(300 * time.Millisecond)
	for i := 0; i < 5; i++ {
		send(0x90, 60, 97, 0x90, 60, 0)
	}
	if n := count(200 * time.Millisecond); n != 1 {
		t.Fatalf("5 rapid presses gave %d triggers, want 1 (debounce)", n)
	}

	// After the debounce window, it triggers again — via running status this time.
	time.Sleep(300 * time.Millisecond)
	send(0x90, 60, 97)
	send(60, 0) // running status note-off
	if n := count(150 * time.Millisecond); n != 1 {
		t.Fatalf("press after debounce gave %d triggers, want 1", n)
	}
}
