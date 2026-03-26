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
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

type biquadFilter struct {
	b0, b1, b2 float64
	a1, a2     float64
	z1, z2     float64
}

func newVoiceBandpassFilter(sampleRate, lowHz, highHz float64) *biquadFilter {
	if lowHz <= 0 {
		lowHz = 300
	}
	if highHz <= lowHz {
		highHz = lowHz * 2
	}
	center := math.Sqrt(lowHz * highHz)
	bandwidth := highHz - lowHz
	if bandwidth <= 0 {
		bandwidth = center
	}
	q := center / bandwidth
	if q <= 0 {
		q = 0.5
	}
	omega := 2 * math.Pi * center / sampleRate
	alpha := math.Sin(omega) / (2 * q)
	cosw := math.Cos(omega)
	b0 := alpha
	b1 := 0.0
	b2 := -alpha
	a0 := 1 + alpha
	a1 := -2 * cosw
	a2 := 1 - alpha
	return &biquadFilter{
		b0: b0 / a0,
		b1: b1 / a0,
		b2: b2 / a0,
		a1: a1 / a0,
		a2: a2 / a0,
	}
}

func (f *biquadFilter) Process(samples []float32) {
	for i := range samples {
		input := float64(samples[i])
		output := f.b0*input + f.z1
		f.z1 = f.b1*input + f.z2 - f.a1*output
		f.z2 = f.b2*input - f.a2*output
		samples[i] = float32(output)
	}
}

type VTTService struct {
	// Audio configuration
	rate      float64
	chunkSize int
	channels  int
	language  string

	// PortAudio
	stream      *portaudio.Stream
	inputDevice *portaudio.DeviceInfo

	// Voice bandpass filter (300-3400 Hz)
	voiceFilter *biquadFilter

	// Whisper model
	whisperModel *WhisperModel

	// Threading and control
	mutex sync.RWMutex

	// Internal control
	stopChan  chan struct{}
	AudioData chan []float32

	// Noise gate: minimum RMS level to allow audio into the VAD pipeline (0 = disabled)
	noiseGateThreshold float32

	// Crest factor threshold: frames with peak/RMS above this are percussive transients (0 = disabled)
	crestFactorMax float64

	// Minimum speech buffer duration in milliseconds before dispatch (0 = disabled)
	minSpeechMs int

	// Dictation status
	DictationEnabled bool

	// Track if last sent output was a newline to avoid leading space
	lastSentNewline bool
}

func NewVTTSrv() (*VTTService, error) {
	// Load .env file if present
	_ = godotenv.Load()

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

	// Read crest factor max from environment variable
	crestFactorMax := 8.0
	if val := os.Getenv("VTT_CREST_FACTOR_MAX"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			crestFactorMax = parsed
			fmt.Printf("Crest factor max set to: %f\n", crestFactorMax)
		} else {
			log.Printf("Warning: invalid VTT_CREST_FACTOR_MAX value %q, using default 8.0", val)
		}
	}

	// Read minimum speech duration from environment variable
	minSpeechMs := 300
	if val := os.Getenv("VTT_MIN_SPEECH_MS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			minSpeechMs = parsed
			fmt.Printf("Minimum speech duration set to: %d ms\n", minSpeechMs)
		} else {
			log.Printf("Warning: invalid VTT_MIN_SPEECH_MS value %q, using default 300ms", val)
		}
	}

	service := &VTTService{
		rate:               16000,
		chunkSize:          2048,
		channels:           1,
		language:           "es",
		stopChan:           make(chan struct{}),
		noiseGateThreshold: noiseGate,
		crestFactorMax:     crestFactorMax,
		minSpeechMs:        minSpeechMs,
	}
	service.voiceFilter = newVoiceBandpassFilter(service.rate, 300, 3400)

	// Find input device

	service.inputDevice, err = service.findInpDev(devnam)
	if err != nil {
		portaudio.Terminate()
		return nil, err
	}

	// Load Whisper model
	fmt.Println("Loading Whisper model...")
	modelpth := os.Getenv("MODEL")
	if modelpth == "" {
		modelpth = "./vtt_models/ggml-base.bin"
	}
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

	vtt.voiceFilter.Process(in)

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

// SetLanguage sets the transcription language in a thread-safe manner.
func (vtt *VTTService) SetLanguage(lang string) {
	vtt.mutex.Lock()
	defer vtt.mutex.Unlock()
	vtt.language = lang
}

// GetLanguage returns the current transcription language in a thread-safe manner.
func (vtt *VTTService) GetLanguage() string {
	vtt.mutex.RLock()
	defer vtt.mutex.RUnlock()
	return vtt.language
}

// SetDictation sets the dictation (mute) state in a thread-safe manner.
func (vtt *VTTService) SetDictation(enabled bool) {
	vtt.mutex.Lock()
	defer vtt.mutex.Unlock()
	vtt.DictationEnabled = enabled
}

// GetDictation returns the current dictation (mute) state in a thread-safe manner.
func (vtt *VTTService) GetDictation() bool {
	vtt.mutex.RLock()
	defer vtt.mutex.RUnlock()
	return vtt.DictationEnabled
}

// ToggleDictation flips the dictation state and returns the new state.
func (vtt *VTTService) ToggleDictation() bool {
	vtt.mutex.Lock()
	defer vtt.mutex.Unlock()
	vtt.DictationEnabled = !vtt.DictationEnabled
	return vtt.DictationEnabled
}

// computeCrestFactor returns peak absolute value / RMS for the samples.
// Returns 0 if RMS is zero.
func computeCrestFactor(samples []float32) float64 {
	var sumSq float64
	var peak float64
	for _, s := range samples {
		v := float64(s)
		sumSq += v * v
		if abs := math.Abs(v); abs > peak {
			peak = abs
		}
	}
	rms := math.Sqrt(sumSq / float64(len(samples)))
	if rms == 0 {
		return 0
	}
	return peak / rms
}

// computeZCR returns the zero crossing rate: number of sign changes divided by (len-1).
// Returns a value in [0, 1].
func computeZCR(samples []float32) float64 {
	if len(samples) < 2 {
		return 0
	}
	crossings := 0
	for i := 1; i < len(samples); i++ {
		if (samples[i] >= 0) != (samples[i-1] >= 0) {
			crossings++
		}
	}
	return float64(crossings) / float64(len(samples)-1)
}

// isTransient returns true if the frame is a percussive transient (not voice).
// Uses crest factor as primary signal and ZCR as corroboration.
func (vtt *VTTService) isTransient(frame []float32) bool {
	if vtt.crestFactorMax <= 0 {
		return false
	}
	cf := computeCrestFactor(frame)
	if cf > vtt.crestFactorMax {
		zcr := computeZCR(frame)
		// High ZCR corroborates broadband transient; low ZCR may be a loud voiced consonant
		if zcr > 0.3 {
			return true
		}
	}
	return false
}
