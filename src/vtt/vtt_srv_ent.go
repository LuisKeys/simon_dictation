package vtt

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gen2brain/beeep"
	"github.com/gordonklaus/portaudio"
	"github.com/pkg/errors"
)

type VTTService struct {
	// Audio configuration
	rate      float64
	chunkSize int
	channels  int
	language  string

	// PortAudio
	stream      *portaudio.Stream
	inputDevice *portaudio.DeviceInfo

	// Whisper model
	whisperModel *WhisperModel

	// Threading and control
	mutex sync.RWMutex

	// Internal control
	stopChan  chan struct{}
	AudioData chan []float32

	// Noise gate: minimum RMS level to allow audio into the VAD pipeline (0 = disabled)
	noiseGateThreshold float32

	// Dictation status
	DictationEnabled bool
}

func NewVTTSrv() (*VTTService, error) {
	// Initialize PortAudio
	devnam := os.Getenv("VTT_INPUT_DEVICE")
	err := portaudio.Initialize()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize portaudio")
	}

	// Read noise gate threshold from environment variable
	var noiseGate float32
	if val := os.Getenv("VTT_NOISE_GATE"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 32); err == nil {
			noiseGate = float32(parsed)
			fmt.Printf("Noise gate threshold set to: %f\n", noiseGate)
		} else {
			log.Printf("Warning: invalid VTT_NOISE_GATE value %q, noise gate disabled", val)
		}
	}

	service := &VTTService{
		rate:               16000,
		chunkSize:          2048,
		channels:           1,
		language:           "es",
		stopChan:           make(chan struct{}),
		noiseGateThreshold: noiseGate,
	}

	// Find input device

	service.inputDevice, err = service.findInpDev(devnam)
	if err != nil {
		portaudio.Terminate()
		return nil, err
	}

	// Load Whisper model
	fmt.Println("Loading Whisper model...")
	modelpth := "./vtt_models/ggml-base.bin"
	service.whisperModel = NewWhisperModel(modelpth)
	fmt.Println("Whisper model loaded successfully")

	// Create audio stream
	service.stream, err = portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   service.inputDevice,
			Channels: service.channels,
			Latency:  service.inputDevice.DefaultLowInputLatency,
		},
		SampleRate:      service.rate,
		FramesPerBuffer: service.chunkSize,
	}, service.processAudio)
	if err != nil {
		service.whisperModel.Close()
		portaudio.Terminate()
		return nil, errors.Wrap(err, "failed to open audio stream")
	}

	return service, nil
}

func (vtt *VTTService) findInpDev(name string) (*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}

	if name == "" {
		// Return default input device
		defaultDevice, err := portaudio.DefaultInputDevice()
		if err != nil {
			return nil, err
		}
		return defaultDevice, nil
	}

	for _, d := range devices {
		if d.MaxInputChannels > 0 && strings.Contains(strings.ToLower(d.Name), strings.ToLower(name)) {
			return d, nil
		}
	}

	// Fallback to default if named device not found
	log.Printf("Input device %q not found, using default", name)
	return portaudio.DefaultInputDevice()
}

func (vtt *VTTService) Shutdown() {
	vtt.mutex.Lock()
	defer vtt.mutex.Unlock()

	if vtt.stream != nil {
		vtt.stream.Close()
	}

	if vtt.whisperModel != nil {
		vtt.whisperModel.Close()
	}

	portaudio.Terminate()
}

func (vtt *VTTService) processAudio(in []float32) {
	vtt.mutex.Lock()
	defer vtt.mutex.Unlock()

	select {
	case vtt.AudioData <- in:
	default:
		// Channel is full, skip this chunk to avoid blocking
	}
}

// helper to calculate mean and stddev
func CalcMeanStdDev(samples []float64) (mean, stddev float64) {
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	mean = sum / float64(len(samples))

	varianceSum := 0.0
	for _, v := range samples {
		varianceSum += (v - mean) * (v - mean)
	}
	stddev = math.Sqrt(varianceSum / float64(len(samples)))
	return
}

func Notification(title, message string) error {
	return beeep.Notify(title, message, "")
}
