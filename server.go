package jsonrpc2

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
)

type Server struct {
	shutdown chan struct{}
	once     sync.Once

	runner Runner
}

type Runner interface {
	Run(ctx context.Context, in io.Reader, out io.Writer) error
}

func NewServer(runner Runner) *Server {
	return &Server{
		shutdown: make(chan struct{}),
		runner:   runner,
	}
}

func (s *Server) ServeFromIO(ctx context.Context, in io.ReadCloser, out io.Writer) error {
	go func() {
		<-s.shutdown
		in.Close()
	}()

	return s.runner.Run(ctx, in, out)
}

func (s *Server) ServeFromNetwork(ctx context.Context, network, address string) error {
	lr, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	defer lr.Close()

	// Counts the number of active connections
	var wg sync.WaitGroup

	// Close listener upon shutdown
	go func() {
		<-s.shutdown
		lr.Close()
	}()

	for i := 0; ; i++ {
		conn, err := lr.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				wg.Wait()
				return nil
			}
			return err
		}

		wg.Add(1)

		go func(i int, conn net.Conn) {
			logger.Printf("serving connection %d\n", i)

			defer func() {
				conn.Close()
				wg.Done()
			}()

			go func() {
				<-s.shutdown
				// Close the connection when server is shutting down
				conn.Close()
			}()

			if err := s.runner.Run(ctx, conn, conn); err != nil {
				logger.Printf("error serving connection %d: %s\n", i, err)
			}
		}(i, conn)
	}
}

func (s *Server) Close() error {
	s.once.Do(func() {
		close(s.shutdown)
	})

	return nil
}
