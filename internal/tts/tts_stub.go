//go:build !tts

package tts

import (
	"context"
	"fmt"
)

type stubSpeaker struct{}

func (s *stubSpeaker) Speak(ctx context.Context, text string) error {
	return fmt.Errorf("voice output is not enabled (build with -tags tts)")
}

func (s *stubSpeaker) Wait() error {
	return nil
}

func (s *stubSpeaker) Close() error {
	return nil
}

func newSpeaker() (Speaker, error) {
	return &stubSpeaker{}, nil
}
