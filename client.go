package jsonrpc2

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
)

type Client struct {
	mu sync.Mutex

	currentId int64

	in  io.Reader
	out io.Writer

	requests map[string]chan<- any
}

func NewClient(cmd *exec.Cmd) (*Client, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe Stdout: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe Stdin: %w", err)
	}

	return &Client{
		in:  stdout,
		out: stdin,

		requests: make(map[string]chan<- any),
	}, nil
}

func NewClientFromInOut(in io.Reader, out io.Writer) *Client {
	return &Client{
		in:       in,
		out:      out,
		requests: make(map[string]chan<- any),
	}
}

func (c *Client) Read() error {
	s := bufio.NewScanner(c.in)

	for s.Scan() {
		if err := c.requestReceive(s.Bytes()); err != nil {
			return err
		}
	}

	if err := s.Err(); err != nil {
		return fmt.Errorf("failed to read messages: %w", err)
	}

	return nil
}

func (c *Client) Call(ctx context.Context, method string, req any, resp any) error {
	id := strconv.FormatInt(atomic.AddInt64(&c.currentId, 1), 36)

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
		Id:      json.RawMessage(bid),
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
			return json.Unmarshal(v.Result, resp)
		case error:
			return v
		default:
			panic(fmt.Sprintf("unexpected response type: %T", v))
		}
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (c *Client) requestReceive(message []byte) error {
	var resp rpcResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			// A syntax error is a protocol violation that may result in an undefined
			// behavior, so the error is returned with further termination of the process.
			return err
		}
		// Continue processing messages without terminating the process.
		return nil
	}

	if err := Val.Struct(&resp); err != nil {
		// Continue processing messages without terminating the process.
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var id string
	if err := json.Unmarshal(resp.Id, &id); err != nil {
		// Continue processing messages without terminating the process.
		return nil
	}

	if ch, ok := c.requests[id]; ok {
		ch <- resp
	}

	return nil
}

func (c *Client) requestSend(id string, req rpcRequest) (chan any, error) {
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

	if _, err := fmt.Fprintln(c.out, string(content)); err != nil {
		// Caller is responsible to clean up request upon failure
		return nil, err
	}

	return ch, nil
}

func (c *Client) requestClose(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, ok := c.requests[id]; ok {
		close(ch)
		delete(c.requests, id)
	}
}
