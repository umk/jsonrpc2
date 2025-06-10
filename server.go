package jsonrpc2

import (
	"bufio"
	"context"
	"io"
	"log"
	"sync"

	"github.com/umk/jsonrpc2/internal/slices"
)

type ServerOption func(*serverOptions)

type serverOptions struct {
	requestSize int
}

func WithServerRequestSize(size int) ServerOption {
	return func(opts *serverOptions) {
		opts.requestSize = size
	}
}

type Server struct {
	handler    *Handler
	bufferPool *slices.SlicePool[byte]
}

func NewServer(handler *Handler, opts ...ServerOption) *Server {
	options := &serverOptions{
		requestSize: defaultRequestSize,
	}

	for _, opt := range opts {
		opt(options)
	}

	return &Server{
		handler:    handler,
		bufferPool: slices.NewSlicePool[byte](options.requestSize),
	}
}

func (s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	var wg sync.WaitGroup
	var mu sync.Mutex

	defer wg.Wait()

	reader := bufio.NewReader(in)
	for {
		data := s.bufferPool.Get(0)
		if err := readInput(reader, data); err != nil {
			s.bufferPool.Put(data)
			if err == io.EOF {
				return nil
			}
			return err
		}

		wg.Add(1)
		go func(data *[]byte) {
			defer wg.Done()
			defer s.bufferPool.Put(data)

			resp, err := s.handler.Handle(ctx, *data)
			if err != nil {
				log.Println("Error processing request:", err)
				return
			}

			if resp != nil {
				mu.Lock()
				defer mu.Unlock()

				out.Write(resp)
				out.Write(separator)
			}
		}(data)
	}
}
