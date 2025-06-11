package jsonrpc2

import (
	"bufio"
	"io"
	"sync"
)

var separator = []byte{'\n'}

// messageWriter provides thread-safe writing of JSON-RPC messages
type messageWriter struct {
	mu  sync.Mutex
	out io.Writer
}

// newMessageWriter creates a new message writer
func newMessageWriter(out io.Writer) *messageWriter {
	return &messageWriter{out: out}
}

// Write writes a message followed by a separator
func (w *messageWriter) Write(buf []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.out.Write(buf); err != nil {
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
	return &messageReader{reader: bufio.NewReader(in)}
}

// Read reads a complete line from reader into input buffer
// handling any line prefixes correctly. It accepts reusable buffer
// and returns a line in either the same or a new buffer.
func (r *messageReader) Read(buf []byte) ([]byte, error) {
	buf = buf[:0]

	for proceed := true; proceed; {
		line, isPrefix, err := r.reader.ReadLine()
		if err != nil {
			return nil, err
		}

		buf = append(buf, line...)
		proceed = isPrefix
	}

	return buf, nil
}
