package tts

import (
	"context"
)

// Speaker is the interface for text-to-speech output.
type Speaker interface {
	// Speak converts text to speech and plays it.
	Speak(ctx context.Context, text string) error
	// Wait waits for all queued audio to finish playing.
	Wait() error
	// Close releases resources.
	Close() error
}

// NewSpeaker returns a new Speaker instance if the "tts" build tag is set.
// Otherwise it returns a stub that does nothing.
func NewSpeaker() (Speaker, error) {
	return newSpeaker()
}

// DownloadProgress is a callback for model download progress.
type DownloadProgress func(string)
