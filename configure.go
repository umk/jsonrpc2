package jsonrpc2

import (
	"log"

	"github.com/umk/jsonrpc2/internal/slices"
)

var bufs *slices.SlicePool[byte]

var logger *log.Logger = log.Default()

var currentConf = packageConf{
	requestSize: 4 * 1024,
}

func init() {
	bufs = slices.NewSlicePool[byte](currentConf.requestSize)
}

type Option func(*packageConf)

type packageConf struct {
	requestSize int
	logger      *log.Logger
}

// WithRequestSize sets the request buffer pool size.
func WithRequestSize(size int) Option {
	return func(conf *packageConf) {
		conf.requestSize = size
	}
}

// WithLogger sets the global logger.
func WithLogger(l *log.Logger) Option {
	return func(conf *packageConf) {
		conf.logger = l
	}
}

func Configure(opts ...Option) {
	previousConf := currentConf
	for _, opt := range opts {
		opt(&currentConf)
	}
	if currentConf.requestSize != previousConf.requestSize {
		bufs = slices.NewSlicePool[byte](currentConf.requestSize)
	}
	if currentConf.logger != nil {
		logger = currentConf.logger
	}
}
