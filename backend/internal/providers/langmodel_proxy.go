package providers

import (
	"context"
	"fmt"

	"github.com/odysseythink/pantheon/core"
)

// languageModelProxy implements core.LanguageModel by delegating to the
// current model held by ManagedLLMProvider. It uses acquire/release to
// ensure the underlying provider is not replaced while in use.
type languageModelProxy struct {
	manager *ManagedLLMProvider
}

func (p *languageModelProxy) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	pantheonLLM := p.manager.acquire()
	if pantheonLLM == nil {
		return nil, fmt.Errorf("LLM provider not available")
	}
	defer p.manager.release(pantheonLLM)
	return pantheonLLM.model.Generate(ctx, req)
}

func (p *languageModelProxy) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	pantheonLLM := p.manager.acquire()
	if pantheonLLM == nil {
		return func(yield func(*core.StreamPart, error) bool) {
			yield(nil, fmt.Errorf("LLM provider not available"))
		}, nil
	}
	return func(yield func(*core.StreamPart, error) bool) {
		defer p.manager.release(pantheonLLM)
		stream, err := pantheonLLM.model.Stream(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		for part, err := range stream {
			if !yield(part, err) {
				return
			}
		}
	}, nil
}

func (p *languageModelProxy) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	pantheonLLM := p.manager.acquire()
	if pantheonLLM == nil {
		return nil, fmt.Errorf("LLM provider not available")
	}
	defer p.manager.release(pantheonLLM)
	return pantheonLLM.model.GenerateObject(ctx, req)
}

func (p *languageModelProxy) Provider() string {
	pantheonLLM := p.manager.acquire()
	if pantheonLLM == nil {
		return "unknown"
	}
	defer p.manager.release(pantheonLLM)
	return pantheonLLM.model.Provider()
}

func (p *languageModelProxy) Model() string {
	pantheonLLM := p.manager.acquire()
	if pantheonLLM == nil {
		return "unknown"
	}
	defer p.manager.release(pantheonLLM)
	return pantheonLLM.model.Model()
}
