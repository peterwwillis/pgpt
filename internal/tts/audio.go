package tts

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
)

type audioPlayer struct {
	mctx   *malgo.AllocatedContext
	device *malgo.Device
	mu     sync.Mutex
	queue  [][]float32
}

func newAudioPlayer(sampleRate int) (*audioPlayer, error) {
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}

	p := &audioPlayer{
		mctx: mctx,
	}

	// Log active backend
	if os.Getenv("ZOP_DEBUG_TTS") == "1" {
		// Attempt to get backend name via a small hack or just list
		fmt.Fprintf(os.Stderr, "[zop] tts: malgo context initialized\n")
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = malgo.FormatF32
	cfg.Playback.Channels = 1
	cfg.SampleRate = uint32(sampleRate)
	cfg.Alsa.NoMMap = 1

	if os.Getenv("ZOP_DEBUG_TTS") == "1" {
		fmt.Fprintf(os.Stderr, "[zop] tts: opening playback device at %d Hz\n", sampleRate)
	}

	device, err := malgo.InitDevice(mctx.Context, cfg, malgo.DeviceCallbacks{
		Data: func(pOutput, pInput []byte, frameCount uint32) {
			p.onAudio(pOutput, pInput, frameCount)
		},
	})
	if err != nil {
		_ = mctx.Uninit()
		mctx.Free()
		return nil, err
	}
	p.device = device

	if err := device.Start(); err != nil {
		device.Uninit()
		_ = mctx.Uninit()
		mctx.Free()
		return nil, err
	}

	// Give it a moment to start up
	time.Sleep(100 * time.Millisecond)

	if os.Getenv("ZOP_DEBUG_TTS") == "1" {
		fmt.Fprintf(os.Stderr, "[zop] tts: device started\n")
	}

	return p, nil
}

func (p *audioPlayer) Play(samples []float32) {
	if os.Getenv("ZOP_DEBUG_TTS") == "1" {
		fmt.Fprintf(os.Stderr, "[zop] tts: queuing %d samples\n", len(samples))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queue = append(p.queue, samples)
}

func (p *audioPlayer) Wait() {
	start := time.Now()
	for {
		p.mu.Lock()
		empty := len(p.queue) == 0
		p.mu.Unlock()
		if empty {
			break
		}
		if time.Since(start) > 10*time.Second {
			if os.Getenv("ZOP_DEBUG_TTS") == "1" {
				fmt.Fprintf(os.Stderr, "[zop] tts: wait timeout reached, still have %d chunks in queue\n", len(p.queue))
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (p *audioPlayer) onAudio(pOutput, pInput []byte, frameCount uint32) {
	// ALWAYS clear pOutput (silence) first
	for i := range pOutput {
		pOutput[i] = 0
	}

	if !p.mu.TryLock() {
		return
	}
	defer p.mu.Unlock()

	if len(p.queue) == 0 {
		return
	}

	// Each float32 is 4 bytes.
	totalBytesNeeded := uint32(len(pOutput))
	var bytesWritten uint32

	for bytesWritten < totalBytesNeeded && len(p.queue) > 0 {
		current := p.queue[0]
		bytesInCurrent := uint32(len(current) * 4)
		
		toCopy := totalBytesNeeded - bytesWritten
		if bytesInCurrent < toCopy {
			toCopy = bytesInCurrent
		}

		if toCopy > 0 {
			src := unsafe.Slice((*byte)(unsafe.Pointer(&current[0])), len(current)*4)
			copy(pOutput[bytesWritten:], src[:toCopy])
			
			bytesWritten += toCopy

			if toCopy == bytesInCurrent {
				p.queue = p.queue[1:]
			} else {
				samplesConsumed := toCopy / 4
				p.queue[0] = current[samplesConsumed:]
			}
		} else {
			break
		}
	}
}

func (p *audioPlayer) Close() error {
	p.Wait()
	if p.device != nil {
		p.device.Uninit()
	}
	if p.mctx != nil {
		_ = p.mctx.Uninit()
		p.mctx.Free()
	}
	return nil
}
