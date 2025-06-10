package jsonrpc2

import (
	"bufio"
	"io"
	"sync"
)

var separator = []byte{'\n'}

// messageWriter provides thread-safe writing of JSON-RPC messages
type messageWriter struct {
	out io.Writer
	mu  sync.Mutex
}

// newMessageWriter creates a new message writer
func newMessageWriter(out io.Writer) *messageWriter {
	return &messageWriter{
		out: out,
	}
}

// Write writes a message followed by a separator
func (w *messageWriter) write(message []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.out.Write(message); err != nil {
		return err
	}
	if _, err := w.out.Write(separator); err != nil {
		return err
	}

	return nil
}

// messageReader provides reading of JSON-RPC messages
type messageReader struct {
	reader *bufio.Reader
}

// newMessageReader creates a new message reader
func newMessageReader(in io.Reader) *messageReader {
	return &messageReader{
		reader: bufio.NewReader(in),
	}
}

// read reads a complete line from reader into input buffer
// handling any line prefixes correctly
func (r *messageReader) read(input *[]byte) error {
	*input = (*input)[:0]

	for proceed := true; proceed; {
		line, isPrefix, err := r.reader.ReadLine()
		if err != nil {
			return err
		}

		*input = append(*input, line...)
		proceed = isPrefix
	}

	return nil
}
