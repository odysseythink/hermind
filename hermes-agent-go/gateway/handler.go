package gateway

import (
	"context"
	"log"
)

// DispatchAndReply is a convenience adapters call inside their event
// loop: it runs the MessageHandler and calls SendReply if it succeeds.
// Errors are logged.
func DispatchAndReply(ctx context.Context, p Platform, handler MessageHandler, in IncomingMessage) {
	out, err := handler(ctx, in)
	if err != nil {
		log.Printf("gateway: %s: handler error: %v", p.Name(), err)
		return
	}
	if out == nil {
		return
	}
	if err := p.SendReply(ctx, *out); err != nil {
		log.Printf("gateway: %s: send reply error: %v", p.Name(), err)
	}
}
