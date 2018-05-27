package main

import (
	"github.com/ZhangGuangxu/netbuffer"
)

const (
	headerByteCount   = 4
	protoIDByteCount  = 2
	maxDataLen        = 64 * 1024 * 1024
	checkSumByteCount = 4
	minDataLen        = protoIDByteCount + checkSumByteCount
)

// Codec is a interface that groups Encode,Decode methods and so on.
type Codec interface {
	Unpack(buf *netbuffer.Buffer, client *Client) error
	Pack(out *netbuffer.Buffer, msg *message) error
	Decode(protoID int16, data []byte) (proto interface{}, err error)
	Encode(proto interface{}) (data []byte, err error)
}
