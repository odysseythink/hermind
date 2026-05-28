package processors

import (
	"context"
	"sync"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/stretchr/testify/assert"
)

type mockExtractor struct{}

func (m *mockExtractor) Supports(ext string) bool { return ext == ".mock" }
func (m *mockExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	return &pipeline.ExtractOutput{Content: "mock"}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	m := &mockExtractor{}
	r.Register(".mock", m)

	got := r.Get(".mock")
	assert.NotNil(t, got)
	assert.Nil(t, r.Get(".other"))
}

func TestRegistry_AllExtensions(t *testing.T) {
	r := NewRegistry()
	r.Register(".a", &mockExtractor{})
	r.Register(".b", &mockExtractor{})

	exts := r.AllExtensions()
	assert.Len(t, exts, 2)
	assert.Contains(t, exts, ".a")
	assert.Contains(t, exts, ".b")
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	m := &mockExtractor{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Register(".mock", m)
			_ = r.Get(".mock")
			_ = r.AllExtensions()
		}()
	}
	wg.Wait()

	// Verify final state is correct after concurrent access.
	assert.NotNil(t, r.Get(".mock"))
	exts := r.AllExtensions()
	assert.Len(t, exts, 1)
	assert.Contains(t, exts, ".mock")
}
