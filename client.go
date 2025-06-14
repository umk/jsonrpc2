package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
)

type Client interface {
	Call(ctx context.Context, method string, req any, resp any) error
}

type clientCore struct {
	mu sync.Mutex

	currentID int64 // used to generate unique request IDs

	writer   *messageWriter
	requests map[string]chan<- any // map of request IDs to channels for responses
}

func newClientCore(writer *messageWriter) *clientCore {
	return &clientCore{
		writer:   writer,
		requests: make(map[string]chan<- any),
	}
}

func (c *clientCore) Call(ctx context.Context, method string, req any, resp any) error {
	if err := validateIfStruct(req); err != nil {
		return err
	}

	id := strconv.FormatInt(atomic.AddInt64(&c.currentID, 1), 36)
	id = fmt.Sprintf("%06s", id)

	bid, err := json.Marshal(id)
	if err != nil {
		return err
	}

	breq, err := json.Marshal(req)
	if err != nil {
		return err
	}

	defer c.requestClose(id)

	ch, err := c.requestSend(id, rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  json.RawMessage(breq),
		ID:      json.RawMessage(bid),
	})
	if err != nil {
		return err
	}

	select {
	case r, ok := <-ch:
		if !ok {
			return errors.New("program is terminated")
		}
		switch v := r.(type) {
		case rpcResponse:
			if v.Error != nil {
				return fmt.Errorf("RPC error %d: %s", v.Error.Code, v.Error.Message)
			}
			if err := json.Unmarshal(v.Result, resp); err != nil {
				return err
			}
			return validateIfStruct(resp)
		case error:
			return v
		default:
			panic(fmt.Sprintf("unexpected response type: %T", v))
		}
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (c *clientCore) resolve(message message[rpcResponse]) error {
	var resp rpcResponse
	if err := message.Get(&resp); err != nil {
		return getDispatchError(err)
	}

	var id string
	if err := json.Unmarshal(resp.ID, &id); err != nil {
		// If ID is not a valid string, the request won't be found in the
		// requests map, so can safely discard the response.
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Request is closed by the caller.
	if ch, ok := c.requests[id]; ok {
		ch <- resp
	}

	return nil
}

func (c *clientCore) requestSend(id string, req rpcRequest) (chan any, error) {
	content, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.requests[id]; ok {
		return nil, fmt.Errorf("request %s is already sent", id)
	}

	ch := make(chan any, 1)
	c.requests[id] = ch

	if err := c.writer.Write(content); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	return ch, nil
}

func (c *clientCore) requestClose(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, ok := c.requests[id]; ok {
		delete(c.requests, id)
		close(ch)
	}
}
