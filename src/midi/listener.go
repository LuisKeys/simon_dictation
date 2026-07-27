//go:build linux

// Package midi provides an optional MIDI-note trigger for toggling dictation.
//
// It reads the raw ALSA rawmidi character device (/dev/snd/midiC<card>D<sub>)
// directly, so it needs no cgo and no ALSA/rtmidi/portmidi dependency. The
// listener is opt-in (VTT_MIDI_TOGGLE) and never fatal: if the controller is
// absent, powered off or held by another process, it logs once and keeps
// retrying while the rest of the daemon runs normally.
//
// Linux-only for now. The device discovery and reconnect loop are ALSA
// specific; the stream parser itself is portable if macOS support is added
// later on top of CoreMIDI.
package midi

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultNote     = 60 // C4, middle Do
	defaultDebounce = 250 * time.Millisecond

	anyChannel = -1

	minBackoff = 1 * time.Second
	maxBackoff = 30 * time.Second

	cardsFile = "/proc/asound/cards"
	sndDir    = "/dev/snd"
)

// Config holds the MIDI trigger settings, all sourced from env vars.
type Config struct {
	// Device is either an absolute path to a rawmidi node or a card name
	// fragment (matched against /proc/asound/cards). Empty means autodetect.
	Device string
	// Note is the MIDI note number that toggles dictation.
	Note int
	// Channel restricts the trigger to one MIDI channel (0-15), or
	// anyChannel to accept every channel.
	Channel int
	// Debounce is the minimum interval between two triggers.
	Debounce time.Duration
	// Debug logs every parsed channel message (used to discover note numbers).
	Debug bool
}

// ConfigFromEnv builds a Config from the environment. The second return value
// is false when VTT_MIDI_TOGGLE is unset or 0, meaning the feature is off and
// no device should be opened at all.
//
// Invalid values are discarded in favour of the default rather than being
// fatal, matching the tuning knobs in the vtt package.
func ConfigFromEnv() (Config, bool) {
	if val := os.Getenv("VTT_MIDI_TOGGLE"); val == "" || val == "0" {
		return Config{}, false
	}

	cfg := Config{
		Device:   os.Getenv("VTT_MIDI_DEVICE"),
		Note:     defaultNote,
		Channel:  anyChannel,
		Debounce: defaultDebounce,
	}

	if val := os.Getenv("VTT_MIDI_NOTE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 && parsed <= 127 {
			cfg.Note = parsed
		} else {
			log.Printf("MIDI: invalid VTT_MIDI_NOTE %q, using %d", val, cfg.Note)
		}
	}

	if val := os.Getenv("VTT_MIDI_CHANNEL"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 && parsed <= 15 {
			cfg.Channel = parsed
		} else {
			log.Printf("MIDI: invalid VTT_MIDI_CHANNEL %q, accepting any channel", val)
		}
	}

	if val := os.Getenv("VTT_MIDI_DEBOUNCE_MS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
			cfg.Debounce = time.Duration(parsed) * time.Millisecond
		} else {
			log.Printf("MIDI: invalid VTT_MIDI_DEBOUNCE_MS %q, using %v", val, cfg.Debounce)
		}
	}

	if val := os.Getenv("VTT_MIDI_DEBUG"); val != "" && val != "0" {
		cfg.Debug = true
	}

	return cfg, true
}

// Run opens the MIDI device and dispatches onTrigger every time the configured
// note is pressed. It blocks forever, reconnecting on failure, and is meant to
// be started with `go midi.Run(...)`.
//
// onTrigger runs on this goroutine, so it must not block for long.
func Run(cfg Config, onTrigger func()) {
	chDesc := "any"
	if cfg.Channel != anyChannel {
		chDesc = strconv.Itoa(cfg.Channel)
	}
	log.Printf("MIDI: toggle enabled (note=%d channel=%s debounce=%v)", cfg.Note, chDesc, cfg.Debounce)

	backoff := minBackoff
	// lastFailure dedupes the log while the controller stays unavailable: only
	// a change of failure message is reported, so an unplugged device does not
	// flood the log every retry.
	lastFailure := ""

	reportFailure := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if msg != lastFailure {
			log.Printf("MIDI: %s (retrying every %v)", msg, backoff)
			lastFailure = msg
		}
	}

	for {
		path, err := resolveDevice(cfg.Device)
		if err != nil {
			reportFailure("%v", err)
			time.Sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			// EBUSY here means another process (e.g. a DAW) holds the device:
			// rawmidi nodes are exclusive-open.
			reportFailure("cannot open %s: %v", path, err)
			time.Sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		log.Printf("MIDI: listening on %s", path)
		lastFailure = ""
		backoff = minBackoff

		if err := listen(file, cfg, onTrigger); err != nil {
			reportFailure("%s closed: %v", path, err)
		} else {
			reportFailure("%s closed", path)
		}
		file.Close()

		time.Sleep(backoff)
		backoff = nextBackoff(backoff)
	}
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// listen reads the device until it errors out (unplug, EIO) or hits EOF.
func listen(file *os.File, cfg Config, onTrigger func()) error {
	var p parser
	var lastTrigger time.Time

	reader := bufio.NewReaderSize(file, 256)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}

		note, velocity, channel, ok := p.feed(b)
		if !ok {
			continue
		}

		if cfg.Debug {
			log.Printf("MIDI: note-on ch=%d note=%d vel=%d", channel, note, velocity)
		}

		if note != cfg.Note {
			continue
		}
		if cfg.Channel != anyChannel && channel != cfg.Channel {
			continue
		}
		// Debounce repeated note-ons (key bounce, or a held key retriggering
		// without an intervening note-off) so one press is one toggle.
		if cfg.Debounce > 0 && time.Since(lastTrigger) < cfg.Debounce {
			continue
		}
		lastTrigger = time.Now()

		onTrigger()
	}
}

// resolveDevice returns the rawmidi node path to open. It is called on every
// connection attempt rather than once at startup, because ALSA card numbers
// are reassigned when a USB device is unplugged and plugged back in.
func resolveDevice(device string) (string, error) {
	// An explicit path is used verbatim.
	if strings.HasPrefix(device, "/") {
		return device, nil
	}

	// A name fragment is resolved through /proc/asound/cards to a card number.
	if device != "" {
		card, err := findCardByName(device)
		if err != nil {
			return "", err
		}
		path := filepath.Join(sndDir, "midiC"+strconv.Itoa(card)+"D0")
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("card %d matches %q but has no MIDI port (%s)", card, device, path)
		}
		return path, nil
	}

	// No preference: take the first rawmidi node, in deterministic order.
	matches, err := filepath.Glob(filepath.Join(sndDir, "midiC*D*"))
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("no MIDI device found in %s", sndDir)
	}
	sort.Strings(matches)
	return matches[0], nil
}

// findCardByName matches a fragment case-insensitively against the short id and
// the long name of each card in /proc/asound/cards, whose entries look like:
//
//	4 [mk3            ]: USB-Audio - MicroLab mk3
//	                     Arturia MicroLab mk3 at usb-0000:06:00.3-1.1.2, full speed
func findCardByName(fragment string) (int, error) {
	data, err := os.ReadFile(cardsFile)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %v", cardsFile, err)
	}

	want := strings.ToLower(fragment)
	card := -1
	for _, line := range strings.Split(string(data), "\n") {
		// Header lines start with the card number and an "[id  ]" field;
		// continuation lines are indented and belong to the previous card.
		if num, id, ok := parseCardHeader(line); ok {
			card = num
			if strings.Contains(strings.ToLower(id), want) ||
				strings.Contains(strings.ToLower(line), want) {
				return card, nil
			}
			continue
		}
		if card >= 0 && strings.Contains(strings.ToLower(line), want) {
			return card, nil
		}
	}

	return 0, fmt.Errorf("no sound card matching %q in %s", fragment, cardsFile)
}

// parseCardHeader extracts the card number and short id from a header line of
// /proc/asound/cards, e.g. " 4 [mk3            ]: USB-Audio - MicroLab mk3".
func parseCardHeader(line string) (int, string, bool) {
	lbracket := strings.IndexByte(line, '[')
	rbracket := strings.IndexByte(line, ']')
	if lbracket < 0 || rbracket < lbracket {
		return 0, "", false
	}
	num, err := strconv.Atoi(strings.TrimSpace(line[:lbracket]))
	if err != nil {
		return 0, "", false
	}
	return num, strings.TrimSpace(line[lbracket+1 : rbracket]), true
}

// MIDI status bytes and masks.
const (
	statusFlag = 0x80 // set on status bytes, clear on data bytes

	msgNoteOn = 0x90

	sysExStart = 0xF0
	sysExEnd   = 0xF7
	sysRTStart = 0xF8 // 0xF8-0xFF are single-byte real-time messages
)

// parser is a byte-at-a-time MIDI stream decoder. A raw stream is not a tidy
// sequence of 3-byte messages, so it has to cope with:
//
//   - Running status: a repeated message may omit its status byte, sending
//     bare data bytes that inherit the previous channel status.
//   - System real-time bytes (0xF8-0xFF, notably Active Sensing 0xFE, which
//     many keyboards emit several times a second). These may be interleaved
//     *inside* another message and must not disturb running status.
//   - SysEx (0xF0 ... 0xF7): variable length, skipped wholesale.
//   - Per-type data lengths, which differ between message types.
//
// feed reports only note-on events; everything else is consumed silently.
type parser struct {
	status  byte // current channel status byte (running status), 0 if none
	data    [2]byte
	nData   int // data bytes collected so far
	want    int // data bytes expected for the current status
	inSysEx bool
}

// feed consumes one byte and returns a note-on event when one completes.
// A note-on with velocity 0 is the widely used alias for note-off and is
// reported as consumed but not triggering.
func (p *parser) feed(b byte) (note, velocity, channel int, ok bool) {
	// Single-byte real-time messages can appear anywhere, even mid-message.
	// They must be dropped without touching the running status or the
	// partially collected data bytes.
	if b >= sysRTStart {
		return 0, 0, 0, false
	}

	if b&statusFlag != 0 {
		// A status byte ends any SysEx dump in progress.
		p.inSysEx = false
		p.nData = 0

		if b == sysExStart {
			p.inSysEx = true
			// System messages cancel running status.
			p.status = 0
			return 0, 0, 0, false
		}
		if b >= sysExStart {
			// System common (0xF1-0xF7): cancels running status. 0xF7 is a
			// stray SysEx terminator handled by clearing inSysEx above.
			p.status = 0
			p.want = systemCommonLength(b)
			if p.want > 0 {
				// Reuse the data collector to swallow the payload.
				p.status = b
			}
			return 0, 0, 0, false
		}

		// Channel message: becomes the running status.
		p.status = b
		p.want = channelMessageLength(b)
		return 0, 0, 0, false
	}

	// Data byte.
	if p.inSysEx {
		return 0, 0, 0, false
	}
	if p.status == 0 {
		// Data with no status yet (stream joined mid-message): ignore.
		return 0, 0, 0, false
	}

	p.data[p.nData] = b
	p.nData++
	if p.nData < p.want {
		return 0, 0, 0, false
	}
	p.nData = 0 // running status: the next data bytes start a new message

	if p.status >= sysExStart {
		// Completed a System Common payload; it carries no running status.
		p.status = 0
		return 0, 0, 0, false
	}

	if p.status&0xF0 != msgNoteOn {
		return 0, 0, 0, false
	}
	// Note-on with zero velocity means note-off; not a trigger.
	if p.data[1] == 0 {
		return 0, 0, 0, false
	}
	return int(p.data[0]), int(p.data[1]), int(p.status & 0x0F), true
}

// channelMessageLength returns the number of data bytes a channel message
// carries: 2 for note off/on, aftertouch, control change and pitch bend;
// 1 for program change and channel pressure.
func channelMessageLength(status byte) int {
	switch status & 0xF0 {
	case 0xC0, 0xD0:
		return 1
	default:
		return 2
	}
}

// systemCommonLength returns the number of data bytes for System Common
// messages 0xF1-0xF7.
func systemCommonLength(status byte) int {
	switch status {
	case 0xF1, 0xF3: // MIDI time code quarter frame, song select
		return 1
	case 0xF2: // song position pointer
		return 2
	default: // 0xF4, 0xF5, 0xF6 (tune request), 0xF7
		return 0
	}
}
