package jsonrpc2

import "bufio"

const defaultRequestSize = 4 * 1024

var separator = []byte{'\n'}

// readInput reads a complete line from reader into input buffer
// handling any line prefixes correctly
func readInput(reader *bufio.Reader, input *[]byte) error {
	*input = (*input)[:0]

	for proceed := true; proceed; {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return err
		}

		*input = append(*input, line...)
		proceed = isPrefix
	}

	return nil
}
