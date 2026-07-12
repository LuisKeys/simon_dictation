package vtt

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
)

// vadDebugCounter provides incrementing filenames for dumped utterances.
var vadDebugCounter int64

// dumpUtteranceWAV writes the captured utterance to a 16 kHz mono 16-bit PCM
// WAV file when VAD debugging is enabled. It is best-effort: failures are
// logged but never interrupt the pipeline. The output directory defaults to
// ./vad_debug and can be overridden with VTT_VAD_DEBUG_DIR.
func (vtt *VTTService) dumpUtteranceWAV(samples []float32) {
	dir := os.Getenv("VTT_VAD_DEBUG_DIR")
	if dir == "" {
		dir = "vad_debug"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("VAD debug: cannot create dir %q: %v", dir, err)
		return
	}

	n := atomic.AddInt64(&vadDebugCounter, 1)
	path := filepath.Join(dir, fmt.Sprintf("utterance_%04d.wav", n))
	if err := writeWAV16(path, samples, 16000); err != nil {
		log.Printf("VAD debug: cannot write %q: %v", path, err)
		return
	}
	log.Printf("VAD debug: wrote %s", path)
}

// writeWAV16 writes float32 samples in [-1,1] as a mono 16-bit PCM WAV file.
func writeWAV16(path string, samples []float32, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(samples) * bitsPerSample / 8

	// RIFF header
	if _, err := f.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		return err
	}
	if _, err := f.Write([]byte("WAVE")); err != nil {
		return err
	}

	// fmt chunk
	if _, err := f.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil { // chunk size
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil { // PCM
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// data chunk
	if _, err := f.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}

	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		v := int16(s * 32767)
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	if _, err := f.Write(buf); err != nil {
		return err
	}
	return nil
}
