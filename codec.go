package main

import (
	proto "biblio/protocol"
	"github.com/ZhangGuangxu/netbuffer"
)

const (
	headerByteCount  = 4
	protoIDByteCount = 2
	maxDataLen       = 65536
)

// Codec is a interface that groups Encode,Decode methods and so on.
type Codec interface {
	OnData(buf *netbuffer.Buffer, client *Client) error
	Decode(protoID int16, data []byte) (proto interface{}, err error)
	Encode(proto interface{}) (data []byte, err error)
	proto.ProtoFactory
}
