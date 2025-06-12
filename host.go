package jsonrpc2

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/umk/jsonrpc2/internal/slices"
)

type HostOption func(*hostOptions)

type hostOptions struct {
	writer      *messageWriter // writer for sending responses
	requestSize int            // average size of messages

	client *clientCore // handles client responses
	server *serverCore // handles server requests
}

func WithRequestSize(size int) HostOption {
	return func(opts *hostOptions) {
		opts.requestSize = size
	}
}

func WithClient(client *Client) HostOption {
	return func(opts *hostOptions) {
		c := newClientCore(opts.writer)

		opts.client = c
		*client = c
	}
}

func WithServer(handler *Handler) HostOption {
	return func(opts *hostOptions) {
		opts.server = newServerCore(opts.writer, handler)
	}
}

type Host struct {
	reader     *messageReader
	dispatcher dispatcher
	bufs       *slices.SlicePool[byte]
}

func NewHost(in io.Reader, out io.Writer, opts ...HostOption) *Host {
	options := hostOptions{
		writer:      newMessageWriter(out),
		requestSize: 4 * 1024,
	}

	for _, opt := range opts {
		opt(&options)
	}

	core := newDispatcher(options.client, options.server)
	if core == nil {
		panic("either or both client or server must be specified")
	}

	return &Host{
		reader:     newMessageReader(in),
		dispatcher: core,
		bufs:       slices.NewSlicePool[byte](options.requestSize),
	}
}

func NewHostFromCmd(cmd *exec.Cmd, opts ...HostOption) (*Host, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stdout: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stdin: %w", err)
	}

	h := NewHost(stdout, stdin, opts...)
	return h, nil
}

func (h *Host) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	defer wg.Wait()

	for {
		buf := h.bufs.Get(0)
		b, err := h.reader.Read(*buf)
		if err != nil {
			h.bufs.Put(buf)
			if err == io.EOF {
				return nil
			}
			return err
		}

		buf = &b

		wg.Add(1)
		go func(buf *[]byte) {
			defer wg.Done()
			defer h.bufs.Put(buf)

			if err := h.dispatcher.dispatch(ctx, *buf); err != nil {
				// Do nothing
			}
		}(buf)
	}
}
