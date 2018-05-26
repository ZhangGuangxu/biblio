package main

import (
	proto "biblio/protocol"
	protojson "biblio/protocol/json"
	"encoding/json"
	"errors"
	"github.com/ZhangGuangxu/netbuffer"
)

var errInvalidMsgLength = errors.New("invalid message length")

type jsonCodec struct {
	proto.ProtoFactory
}

func newJSONCodec() *jsonCodec {
	return &jsonCodec{
		ProtoFactory: protojson.ProtoFactory,
	}
}

func (c *jsonCodec) OnData(buf *netbuffer.Buffer, client *Client) error {
	for buf.ReadableBytes() >= headerByteCount {
		length := int(buf.PeekInt32())
		if length > maxDataLen || length < 0 {
			return errInvalidMsgLength
		} else if buf.ReadableBytes() >= headerByteCount+length {
			buf.RetrieveInt32()
			protoID := buf.ReadInt16()
			dataLen := length - protoIDByteCount
			data := buf.PeekAsByteSlice(dataLen)
			proto, err := c.Decode(protoID, data)
			buf.Retrieve(dataLen)
			if err != nil {
				return err
			}

			client.addIncomingMessage(protoID, proto)
		} else {
			break
		}
	}

	return nil
}

func (c *jsonCodec) Decode(protoID int16, data []byte) (interface{}, error) {
	proto, err := c.Require(protoID)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, proto); err != nil {
		return nil, err
	}

	return proto, nil
}

func (c *jsonCodec) Encode(proto interface{}) ([]byte, error) {
	return json.Marshal(proto)
}
