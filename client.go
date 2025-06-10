package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/umk/jsonrpc2/internal/slices"
)

type ClientOption func(*clientOptions)

type clientOptions struct {
	requestSize int
}

func WithClientRequestSize(size int) ClientOption {
	return func(opts *clientOptions) {
		opts.requestSize = size
	}
}

type Client struct {
	mu sync.Mutex

	currentId int64

	reader *messageReader
	writer *messageWriter

	requests   map[string]chan<- any
	bufferPool *slices.SlicePool[byte]
}

func NewClientFromCmd(cmd *exec.Cmd, opts ...ClientOption) (*Client, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe Stdout: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe Stdin: %w", err)
	}

	c := NewClient(stdout, stdin, opts...)
	return c, nil
}

func NewClient(in io.Reader, out io.Writer, opts ...ClientOption) *Client {
	options := &clientOptions{
		requestSize: 4 * 1024,
	}

	for _, opt := range opts {
		opt(options)
	}

	return &Client{
		reader:     newMessageReader(in),
		writer:     newMessageWriter(out),
		requests:   make(map[string]chan<- any),
		bufferPool: slices.NewSlicePool[byte](options.requestSize),
	}
}

func (c *Client) Run() error {
	for {
		data := c.bufferPool.Get(0)
		if err := c.reader.read(data); err != nil {
			c.bufferPool.Put(data)
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to read message: %w", err)
		}

		if err := c.requestReceive(*data); err != nil {
			c.bufferPool.Put(data)
			return err
		}
	}
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

	var id string
	if err := json.Unmarshal(resp.Id, &id); err != nil {
		// Continue processing messages without terminating the process.
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

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

	if err := c.writer.write(content); err != nil {
		delete(c.requests, id)
		close(ch)
		return nil, fmt.Errorf("failed to write request: %w", err)
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
