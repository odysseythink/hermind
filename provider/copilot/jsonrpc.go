package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// rpcResponse is delivered by the read loop into a pending channel.
// Exactly one of Result or Error is non-nil.
type rpcResponse struct {
	Result json.RawMessage
	Error  *rpcError
}

// rpcError mirrors the JSON-RPC 2.0 error shape.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("copilot: rpc error %d: %s", e.Code, e.Message)
}

// call sends a JSON-RPC request and blocks for the matching response.
// ctx cancellation aborts the wait but does not interrupt the child.
func (s *subprocess) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&s.nextID, 1)
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("copilot: marshal params: %w", err)
	}
	frame, err := json.Marshal(struct {
		Version string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{Version: "2.0", ID: id, Method: method, Params: rawParams})
	if err != nil {
		return nil, fmt.Errorf("copilot: marshal frame: %w", err)
	}

	ch := make(chan rpcResponse, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()

	s.writeMu.Lock()
	_, werr := s.stdin.Write(append(frame, '\n'))
	s.writeMu.Unlock()
	if werr != nil {
		s.clear(id)
		return nil, fmt.Errorf("copilot: write: %w", werr)
	}

	select {
	case res := <-ch:
		if res.Error != nil {
			return nil, res.Error
		}
		return res.Result, nil
	case <-ctx.Done():
		s.clear(id)
		return nil, ctx.Err()
	case <-s.closed:
		s.clear(id)
		return nil, fmt.Errorf("copilot: subprocess closed")
	}
}

// readLoop reads newline-delimited JSON-RPC frames off stdout and
// dispatches them to pending calls (by id) or the notification
// bridge (methods without an id).
func (s *subprocess) readLoop() {
	defer func() {
		s.closedFlag.Store(true)
		close(s.closed)
		// Unblock any remaining callers.
		s.mu.Lock()
		for id, ch := range s.pending {
			select {
			case ch <- rpcResponse{Error: &rpcError{Code: -32000, Message: "subprocess closed"}}:
			default:
			}
			delete(s.pending, id)
		}
		s.mu.Unlock()
	}()
	for {
		line, err := s.stdout.ReadBytes('\n')
		if err != nil {
			return
		}
		if len(line) == 0 {
			continue
		}
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Params json.RawMessage `json:"params"`
			Error  *rpcError       `json:"error"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		// Notification: has a method but no id (or id is null).
		if envelope.Method != "" && (len(envelope.ID) == 0 || string(envelope.ID) == "null") {
			select {
			case s.noteBridge <- notification{Method: envelope.Method, Params: envelope.Params}:
			default:
				// drop if no consumer — better than deadlocking the reader
			}
			continue
		}
		// Response: needs an id.
		if len(envelope.ID) == 0 {
			continue
		}
		var id int64
		if err := json.Unmarshal(envelope.ID, &id); err != nil {
			continue
		}
		if id == 0 {
			continue
		}
		s.mu.Lock()
		ch, ok := s.pending[id]
		delete(s.pending, id)
		s.mu.Unlock()
		if !ok {
			continue
		}
		ch <- rpcResponse{Result: envelope.Result, Error: envelope.Error}
	}
}

func (s *subprocess) clear(id int64) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}
