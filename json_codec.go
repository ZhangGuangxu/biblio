package main

import (
	proto "biblio/protocol"
	protojson "biblio/protocol/json"
	"encoding/json"
	"errors"
	"github.com/ZhangGuangxu/netbuffer"
	"hash/adler32"
)

var errInvalidMsgLength = errors.New("invalid message length")
var errChecksumNotMatch = errors.New("checksum not match")

type jsonCodec struct {
	proto.ProtoFactory
}

func newJSONCodec() *jsonCodec {
	return &jsonCodec{
		ProtoFactory: protojson.ProtoFactory,
	}
}

func (c *jsonCodec) Unpack(buf *netbuffer.Buffer, client *Client) error {
	for buf.ReadableBytes() >= headerByteCount {
		length := int(buf.PeekInt32())
		if length > maxDataLen || length < 0 {
			return errInvalidMsgLength
		} else if buf.ReadableBytes() >= headerByteCount+length {
			buf.RetrieveInt32()

			sumLen := length - checkSumByteCount
			s := buf.PeekAsByteSlice(sumLen)
			v1 := adler32.Checksum(s)

			protoID := buf.ReadInt16()

			dataLen := sumLen - protoIDByteCount
			data := buf.PeekAsByteSlice(dataLen)
			proto, err := c.Decode(protoID, data)
			buf.Retrieve(dataLen)
			if err != nil {
				return err
			}

			v2 := buf.ReadInt32()
			if v1 != uint32(v2) {
				return errChecksumNotMatch
			}

			client.addIncomingMessage(protoID, proto)
		} else {
			break
		}
	}

	return nil
}

func (c *jsonCodec) Pack(out *netbuffer.Buffer, msg *message) error {
	data, err := c.Encode(msg.proto)
	if err != nil {
		return err
	}
	err = c.Release(msg.protoID, msg.proto)
	if err != nil {
		return err
	}

	sumLen := protoIDByteCount + len(data)
	msgLen := sumLen + checkSumByteCount

	buf := netbuffer.NewBufferWithSize(msgLen)
	buf.AppendInt16(msg.protoID)
	buf.Append(data)

	s := buf.PeekAsByteSlice(sumLen)
	v := adler32.Checksum(s)
	buf.AppendInt32(int32(v))

	buf.PrependInt32(int32(msgLen))

	out.Append(buf.PeekAllAsByteSlice())
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
