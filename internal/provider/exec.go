package provider

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/peterwwillis/zop/internal/config"
)

type execProvider struct {
	name string
	cfg  config.ProviderConfig
}

func newExecProvider(name string, cfg config.ProviderConfig) *execProvider {
	return &execProvider{
		name: name,
		cfg:  cfg,
	}
}

func (p *execProvider) Name() string { return p.name }

func (p *execProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Construct the full conversation text for context.
	var fullConversation strings.Builder
	var lastPrompt string

	// Prepend model's system prompt if available
	if req.Model.SystemPrompt != "" {
		fullConversation.WriteString("SYSTEM: " + req.Model.SystemPrompt + "\n")
	}

	for i, m := range req.Messages {
		fullConversation.WriteString(strings.ToUpper(m.Role) + ": " + m.Content + "\n")
		if i == len(req.Messages)-1 && m.Role == "user" {
			lastPrompt = m.Content
		}
	}

	// Construct args
	args := append([]string{}, p.cfg.Args...)

	// Add model if flag is set
	if p.cfg.ModelFlag != "" && req.Model.ModelID != "" {
		args = append(args, p.cfg.ModelFlag, req.Model.ModelID)
	}

	// Determine what goes into the prompt flag.
	// If we're using STDIN for the full conversation, the prompt flag should just be the last prompt.
	// If we're not using STDIN, the prompt flag should contain the full conversation.
	var promptArg string
	if p.cfg.UseStdin {
		promptArg = lastPrompt
	} else {
		promptArg = fullConversation.String()
	}

	if p.cfg.PromptFlag != "" {
		args = append(args, p.cfg.PromptFlag, promptArg)
	} else if !p.cfg.UseStdin {
		// If no prompt flag and no STDIN, assume prompt is a positional argument.
		args = append(args, promptArg)
	}

	cmd := exec.CommandContext(ctx, p.cfg.Command, args...)

	// Set up STDIN if requested.
	if p.cfg.UseStdin {
		cmd.Stdin = strings.NewReader(fullConversation.String())
	}

	if req.Stream && req.StreamFunc != nil {
		return p.streamCommand(cmd, req.StreamFunc)
	}
	return p.runCommand(cmd)
}

func (p *execProvider) runCommand(cmd *exec.Cmd) (CompletionResponse, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("command %q failed: %w (output: %s)", cmd.Path, err, string(out))
	}
	return CompletionResponse{Content: string(out)}, nil
}

func (p *execProvider) streamCommand(cmd *exec.Cmd, fn func(string)) (CompletionResponse, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CompletionResponse{}, err
	}
	cmd.Stderr = cmd.Stdout // combine them

	if err := cmd.Start(); err != nil {
		return CompletionResponse{}, err
	}

	var full strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			full.WriteString(chunk)
			fn(chunk)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return CompletionResponse{}, err
		}
	}

	if err := cmd.Wait(); err != nil {
		return CompletionResponse{}, err
	}

	return CompletionResponse{Content: full.String()}, nil
}
