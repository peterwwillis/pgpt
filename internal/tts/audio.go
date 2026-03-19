package tts

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
)

var (
	mctx   *malgo.AllocatedContext
	mctxMu sync.Mutex
)

func getMalgoContext() (*malgo.AllocatedContext, error) {
	mctxMu.Lock()
	defer mctxMu.Unlock()
	if mctx != nil {
		return mctx, nil
	}
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	mctx = ctx
	return mctx, nil
}

type audioPlayer struct {
	mctx   *malgo.AllocatedContext
	device *malgo.Device
	mu     sync.Mutex
	queue  [][]float32
	done   chan struct{}
}

func newAudioPlayer(sampleRate int) (*audioPlayer, error) {
	ctx, err := getMalgoContext()
	if err != nil {
		return nil, err
	}

	p := &audioPlayer{
		mctx: ctx,
		done: make(chan struct{}),
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatF32
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = uint32(sampleRate)

	callbacks := malgo.DeviceCallbacks{
		Data: p.onAudio,
	}

	device, err := malgo.InitDevice(p.mctx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, err
	}

	p.device = device

	if err := device.Start(); err != nil {
		device.Uninit()
		return nil, err
	}

	if os.Getenv("ZOP_DEBUG_TTS") == "1" {
		fmt.Fprintf(os.Stderr, "[zop] tts: device started, sample rate %d\n", sampleRate)
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

func (p *audioPlayer) onAudio(_, output []byte, frameCount uint32) {
	if len(output) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) == 0 {
		return
	}

	if os.Getenv("ZOP_DEBUG_TTS") == "1" {
		// Only log occasionally to avoid flooding
		if time.Now().UnixNano()%100 == 0 {
			fmt.Fprintf(os.Stderr, "[zop] tts: playing audio callback, frameCount=%d, queue_len=%d\n", frameCount, len(p.queue))
		}
	}

	// malgo uses byte slices, we need to treat them as float32
	// each float32 is 4 bytes
	out := (*[1 << 30]float32)(unsafe.Pointer(&output[0]))[:frameCount:frameCount]

	var framesWritten uint32
	for framesWritten < frameCount && len(p.queue) > 0 {
		current := p.queue[0]
		toWrite := frameCount - framesWritten
		if uint32(len(current)) < toWrite {
			toWrite = uint32(len(current))
		}

		copy(out[framesWritten:], current[:toWrite])
		framesWritten += toWrite

		if toWrite == uint32(len(current)) {
			p.queue = p.queue[1:]
		} else {
			p.queue[0] = current[toWrite:]
		}
	}
}

func (p *audioPlayer) Close() error {
	if p.device != nil {
		p.device.Uninit()
	}
	return nil
}
