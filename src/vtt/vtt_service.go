package vtt

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

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
	vtt.DictationEnabled = true
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

			buffer = append(buffer, frame...)

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
					collectingNoise = false
					noiseSamples = nil // free memory
					fmt.Printf("Silence threshold set to: %f\n", silenceThreshold)
				}
			}

			now := time.Now()
			if rms > silenceThreshold {
				speaking = true
				lastSoundTime = now
			} else if speaking && now.Sub(lastSoundTime) > silenceDuration {
				vtt.dispatch(buffer)
				buffer = nil
				speaking = false
			}

		case <-ticker.C:
		}
	}
}

func (vtt *VTTService) dispatch(audioData []float32) {
	if len(audioData) == 0 {
		return
	}
	go func(data []float32) {
		text, err := vtt.whisperModel.Transcribe(data, vtt.language)
		if err != nil {
			log.Printf("transcription error: %v", err)
			return
		}
		text = normalizeText(text)
		if text != "" {
			log.Printf("Transcribed text: %s", text)
			iscmd := Commands(vtt, text)
			if vtt.DictationEnabled && !iscmd {
				input.Send(text)
				//log.Printf("Sent text: %s", text)
			}
		}
	}(append([]float32(nil), audioData...)) // copy to avoid races
}

func normalizeText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, ".", "")
	if len(text) > 0 {
		text = strings.ToLower(string(text[0])) + text[1:]
	}

	text = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' ||
			r == 'á' || r == 'é' || r == 'í' || r == 'ó' || r == 'ú' || r == 'ü' || r == 'ñ' ||
			r == 'Á' || r == 'É' || r == 'Í' || r == 'Ó' || r == 'Ú' || r == 'Ü' || r == 'Ñ' {
			return r
		}
		return -1
	}, text)

	text = strings.TrimSpace(text)
	text = " " + text
	return text
}
