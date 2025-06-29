package jsonrpc2

import (
	"context"
	"io"
	"sync"
)

type HostOption func(*hostOptions)

type hostOptions struct {
	writer      *messageWriter // writer for sending responses
	requestSize int            // average size of messages

	client *clientCore // handles client responses
	server *serverCore // handles server requests
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
	}
}

func (h *Host) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	defer wg.Wait()

	for {
		buf := bufs.Get(0)
		if err := h.reader.Read(buf); err != nil {
			bufs.Put(buf)
			if err == io.EOF {
				return nil
			}
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer bufs.Put(buf)

			if err := h.dispatcher.dispatch(ctx, *buf); err != nil {
				// Do nothing
			}
		}()
	}
}
