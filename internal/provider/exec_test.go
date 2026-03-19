package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/peterwwillis/zop/internal/config"
)

func TestExecProvider_Complete(t *testing.T) {
	// We'll use 'echo' as a mock command to test the provider.
	cfg := config.ProviderConfig{
		Command:    "echo",
		PromptFlag: "",
		Args:       []string{"-n"}, // don't add newline so we can test exact output
	}
	p := newExecProvider("test-cli", cfg)

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "hello world"},
		},
		Model: config.ModelConfig{ModelID: "test-model"},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Echo will output the prompt. Our current logic for no PromptFlag and no UseStdin
	// is to append the prompt as a positional argument.
	// The prompt is "USER: hello world\n"
	expected := "USER: hello world\n"
	if resp.Content != expected {
		t.Errorf("expected %q, got %q", expected, resp.Content)
	}
}

func TestExecProvider_UseStdin(t *testing.T) {
	// 'cat' will output its STDIN.
	cfg := config.ProviderConfig{
		Command:  "cat",
		UseStdin: true,
	}
	p := newExecProvider("test-cli", cfg)

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "hello cat"},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	expected := "USER: hello cat\n"
	if resp.Content != expected {
		t.Errorf("expected %q, got %q", expected, resp.Content)
	}
}

func TestExecProvider_Stream(t *testing.T) {
	// 'echo' doesn't really stream but we can test the pipe logic.
	cfg := config.ProviderConfig{
		Command: "echo",
		Args:    []string{"-n"},
	}
	p := newExecProvider("test-cli", cfg)

	var streamed strings.Builder
	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "streaming test"},
		},
		Stream: true,
		StreamFunc: func(chunk string) {
			streamed.WriteString(chunk)
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	expected := "USER: streaming test\n"
	if streamed.String() != expected {
		t.Errorf("streamed: expected %q, got %q", expected, streamed.String())
	}
	if resp.Content != expected {
		t.Errorf("resp: expected %q, got %q", expected, resp.Content)
	}
}
