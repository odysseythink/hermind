package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/odysseythink/hermind/internal/jsonrpc"
)

// Run reads frames from r and writes responses to w until ctx is
// cancelled or r hits EOF.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.RunOnce(ctx, r, w, -1)
}

// RunOnce is a test seam — stops after n frames (pass -1 for EOF).
func (s *Server) RunOnce(ctx context.Context, r io.Reader, w io.Writer, n int) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<16), 1<<22)
	var writeMu sync.Mutex
	count := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		s.dispatch(ctx, append([]byte{}, line...), w, &writeMu)
		count++
		if n > 0 && count >= n {
			return nil
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, raw []byte, w io.Writer, mu *sync.Mutex) {
	req, err := jsonrpc.DecodeRequest(raw)
	if err != nil {
		s.writeLocked(mu, w, &jsonrpc.Response{
			Error: &jsonrpc.Error{Code: jsonrpc.CodeParseError, Message: err.Error()},
		})
		return
	}

	result, handlerErr := s.route(ctx, req.Method, req.Params)
	if req.IsNotification() {
		return
	}
	resp := &jsonrpc.Response{ID: req.ID}
	if handlerErr != nil {
		resp.Error = &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: handlerErr.Error(),
		}
		if _, ok := handlerErr.(*unknownMethodError); ok {
			resp.Error.Code = jsonrpc.CodeMethodNotFound
		}
	} else {
		resp.Result = result
	}
	s.writeLocked(mu, w, resp)
}

func (s *Server) writeLocked(mu *sync.Mutex, w io.Writer, resp *jsonrpc.Response) {
	mu.Lock()
	defer mu.Unlock()
	_ = jsonrpc.EncodeResponse(w, resp)
}

type unknownMethodError struct{ method string }

func (e *unknownMethodError) Error() string { return "method not found: " + e.method }

func (s *Server) route(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "initialize":
		return s.handleInitialize(ctx, params)
	case "tools/list":
		return s.handleToolsList(ctx, params)
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	}
	return nil, &unknownMethodError{method: method}
}
