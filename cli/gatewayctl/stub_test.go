package gatewayctl_test

import (
	"context"

	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/gateway/platforms"
)

func init() {
	platforms.Register(platforms.Descriptor{
		Type:        "stub",
		DisplayName: "Stub (test-only)",
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return newStubPlatform(opts["name"]), nil
		},
	})
}

type stubPlatform struct{ name string }

func newStubPlatform(name string) *stubPlatform { return &stubPlatform{name: name} }
func (s *stubPlatform) Name() string             { return s.name }
func (s *stubPlatform) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return ctx.Err()
}
func (s *stubPlatform) SendReply(_ context.Context, _ gateway.OutgoingMessage) error { return nil }
