package vtt

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/luiskeys/simon_dictate/src/input"
)

func Init() *VTTService {
	vttsrv, err := NewVTTSrv()
	if err != nil {
		log.Fatal("Error initializing VTT service:", err)
		return nil
	}

	log.Println("VTT service started successfully")

	return vttsrv
}

func (vtt *VTTService) Run() {
	vtt.SetDictation(true)
	vtt.AudioData = make(chan []float32, 8)
	err := vtt.stream.Start()

	if err != nil {
		log.Fatal("Error starting audio stream:", err)
		return
	}

	go vtt.Listen()
	for {
		time.Sleep(10 * time.Millisecond)
	}
}

func (vtt *VTTService) Listen() {
	var (
		buffer           []float32
		speaking         bool
		lastSoundTime    = time.Now()
		noiseSamples     []float64
		silenceThreshold float64
	)

	frameSize := 1024
	sampleRate := 16000
	silenceDuration := 500 * time.Millisecond
	silenceThreshold = 0.01

	// Duration to collect noise samples before starting detection
	noiseSampleDuration := 1 * time.Second
	noiseSampleFrames := int(noiseSampleDuration / (time.Duration(frameSize*1000/sampleRate) * time.Millisecond))

	ticker := time.NewTicker(time.Duration(frameSize*1000/sampleRate) * time.Millisecond)
	defer ticker.Stop()

	collectingNoise := true

	for {
		time.Sleep(1 * time.Millisecond)
		select {
		case frame, ok := <-vtt.AudioData:
			if !ok {
				return
			}

			var sum float64
			for _, s := range frame {
				sum += float64(s * s)
			}
			rms := math.Sqrt(sum / float64(len(frame)))

			if collectingNoise {
				noiseSamples = append(noiseSamples, rms)
				if len(noiseSamples) >= noiseSampleFrames {
					mean, stddev := CalcMeanStdDev(noiseSamples)
					silenceThreshold = (mean + 2*stddev) * 15
					if silenceThreshold > 0.01 {
						// Retry collecting noise samples
						noiseSamples = nil
						fmt.Println("Silence threshold too high, retrying noise collection...")
						continue
					}
					collectingNoise = false
					noiseSamples = nil // free memory
					fmt.Printf("Silence threshold set to: %f\n", silenceThreshold)
				}
			}

			// Noise gate: frames below threshold are treated as absolute silence
			// and are never appended to the speech buffer.
			gated := vtt.noiseGateThreshold > 0 && rms < float64(vtt.noiseGateThreshold)

			now := time.Now()
			isVoiceActive := !collectingNoise && !gated && rms > silenceThreshold && !vtt.isTransient(frame) && !vtt.isAperiodic(frame)
			if isVoiceActive {
				speaking = true
				lastSoundTime = now
				buffer = append(buffer, frame...)
			} else if speaking {
				if !gated {
					buffer = append(buffer, frame...)
				}
				if now.Sub(lastSoundTime) > silenceDuration {
					vtt.dispatch(buffer)
					buffer = nil
					speaking = false
				}
			} else {
				buffer = nil // pure silence, discard accumulated data
			}

		case <-ticker.C:
		}
	}
}

func (vtt *VTTService) dispatch(audioData []float32) {
	if len(audioData) == 0 {
		return
	}
	if vtt.minSpeechMs > 0 {
		minSamples := vtt.minSpeechMs * 16 // 16000 Hz / 1000 ms
		if len(audioData) < minSamples {
			return
		}
	}
	lang := vtt.GetLanguage()
	go func(data []float32, lang string) {
		text, err := vtt.whisperModel.Transcribe(data, lang)
		if err != nil {
			log.Printf("transcription error: %v", err)
			return
		}
		text = normalizeText(text)
		if vtt.nameCapitalizer != nil {
			text = vtt.nameCapitalizer.Apply(text)
		}
		if text != "" {
			log.Printf("Transcribed text: %s", text)
			iscmd, cmdText := Commands(vtt, text)
			if iscmd {
				if cmdText != "" {
					// Use blocking send for commands to ensure ordering with state updates
					_ = input.SendSync(cmdText)
					vtt.mutex.Lock()
					if cmdText == "\n" {
						vtt.lastSentNewline = true
					} else {
						vtt.lastSentNewline = false
					}
					vtt.mutex.Unlock()
				} else {
					vtt.mutex.Lock()
					vtt.lastSentNewline = false
					vtt.mutex.Unlock()
				}
			} else if vtt.GetDictation() {
				vtt.mutex.Lock()
				sendText := text
				if !vtt.lastSentNewline {
					sendText = " " + text
				}
				vtt.lastSentNewline = false
				vtt.mutex.Unlock()
				input.Send(sendText)
				//log.Printf("Sent text: %s", sendText)
			}
		}
	}(append([]float32(nil), audioData...), lang) // copy to avoid races
}

func normalizeText(text string) string {
	text = strings.TrimSpace(text)
	text = lowerFirstLetter(text)
	text = strings.ReplaceAll(text, ".", "")

	text = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' ||
			r == '-' || r == '\'' ||
			r == 'á' || r == 'é' || r == 'í' || r == 'ó' || r == 'ú' || r == 'ü' || r == 'ñ' ||
			r == 'Á' || r == 'É' || r == 'Í' || r == 'Ó' || r == 'Ú' || r == 'Ü' || r == 'Ñ' {
			return r
		}
		return -1
	}, text)

	text = removeNoiseWords(text)

	text = strings.TrimSpace(text)
	return text
}

func lowerFirstLetter(text string) string {
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if unicode.IsLetter(runes[i]) {
			runes[i] = unicode.ToLower(runes[i])
			return string(runes)
		}
	}
	return text
}

func removeNoiseWords(text string) string {
	noise := map[string]struct{}{
		"music": {}, "coughing": {}, "laughing": {},
		"musica": {}, "música": {}, "risas": {}, "risa": {}, "tos": {},
	}

	tokens := tokenizeText(text)
	if len(tokens) == 0 {
		return text
	}

	var b strings.Builder
	b.Grow(len(text))
	for _, t := range tokens {
		if t.kind == tokenWord {
			if _, drop := noise[normalizeKey(t.text)]; drop {
				continue
			}
		}
		b.WriteString(t.text)
	}
	return b.String()
}
