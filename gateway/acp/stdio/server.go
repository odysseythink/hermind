package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Server drives the stdio read/dispatch loop.
type Server struct {
	handlers *Handlers

	// writeMu serializes stdout writes so concurrent prompt handlers
	// and notification senders don't interleave JSON frames.
	writeMu sync.Mutex
}

// NewServer constructs a server with the given handler bundle.
func NewServer(h *Handlers) *Server {
	return &Server{handlers: h}
}

// Run reads frames from r and writes responses to w until r hits EOF
// or ctx is cancelled. Non-fatal decode errors are reported as
// parse-error responses; they don't abort the loop.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.RunOnce(ctx, r, w, -1)
}

// RunOnce is a test seam that stops after dispatching n frames
// (pass -1 for "until EOF").
func (s *Server) RunOnce(ctx context.Context, r io.Reader, w io.Writer, n int) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<16), 1<<22) // up to 4 MiB per frame
	count := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Copy the scanner's internal buffer — bufio reuses it.
		frame := append([]byte(nil), line...)
		s.dispatch(ctx, frame, w)
		count++
		if n > 0 && count >= n {
			return nil
		}
	}
	return scanner.Err()
}

// dispatch decodes a single frame, routes it, and writes the response
// (unless the frame is a notification).
func (s *Server) dispatch(ctx context.Context, raw []byte, w io.Writer) {
	req, err := DecodeRequest(raw)
	if err != nil {
		s.write(w, &Response{
			Error: &Error{Code: CodeParseError, Message: err.Error()},
		})
		return
	}

	result, routeErr := s.route(ctx, req.Method, req.Params)
	if req.IsNotification() {
		// Notifications take no response. Side effects from route() still applied.
		return
	}
	resp := &Response{ID: req.ID}
	if routeErr != nil {
		code := CodeInternalError
		var rerr *routingError
		if asRoutingError(routeErr, &rerr) {
			code = CodeMethodNotFound
		}
		resp.Error = &Error{Code: code, Message: routeErr.Error()}
	} else {
		resp.Result = result
	}
	s.write(w, resp)
}

// route dispatches to the right handler by method name. Method names
// mirror both the snake_case shape used by the Python acp library and
// the slash-separated shape defined by the Zed ACP spec; we accept
// either so any conforming client works.
func (s *Server) route(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "initialize":
		return s.handlers.handleInitialize(ctx, params)
	case "authenticate":
		return s.handlers.handleAuthenticate(ctx, params)
	case "session/new", "newSession", "new_session":
		return s.handlers.handleNewSession(ctx, params)
	case "session/load", "loadSession", "load_session":
		return s.handlers.handleLoadSession(ctx, params)
	case "session/prompt", "prompt":
		return s.handlers.handlePrompt(ctx, params)
	case "session/cancel", "cancel":
		return s.handlers.handleCancel(ctx, params)
	}
	return nil, &routingError{method: method}
}

// routingError is the distinguished error type that tells dispatch to
// emit a JSON-RPC -32601 (method not found) code.
type routingError struct{ method string }

func (e *routingError) Error() string { return fmt.Sprintf("method not found: %s", e.method) }

// asRoutingError unwraps using errors.As semantics, but keeps the
// dependency surface minimal. Writing this as a tiny function lets us
// avoid an errors.As import for a single use site.
func asRoutingError(err error, target **routingError) bool {
	if err == nil {
		return false
	}
	if re, ok := err.(*routingError); ok {
		*target = re
		return true
	}
	// Fall back to message prefix — keeps behavior consistent if the
	// handler ever wraps the error.
	const prefix = "method not found: "
	if strings.HasPrefix(err.Error(), prefix) {
		*target = &routingError{method: err.Error()[len(prefix):]}
		return true
	}
	return false
}

// write serializes writes to w so concurrent notification emitters
// don't interleave JSON lines. It swallows write errors — stdout
// closing is terminal and the caller's read loop will exit next turn.
func (s *Server) write(w io.Writer, resp *Response) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = EncodeResponse(w, resp)
}

// WriteNotification emits a server-initiated notification (e.g. a
// session/update frame) with the same serialization guarantees as
// response writes.
func (s *Server) WriteNotification(w io.Writer, method string, params interface{}) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return EncodeNotification(w, method, params)
}
