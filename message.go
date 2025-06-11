package jsonrpc2

import "encoding/json"

type message[M any] interface {
	Get(message *M) error
}

type messageBuf[M any] []byte

func (m messageBuf[M]) Get(message *M) error {
	return json.Unmarshal(m, message)
}

type messageVal[M any] struct {
	message M
}

func (m messageVal[M]) Get(message *M) error {
	*message = m.message
	return nil
}
