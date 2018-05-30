package main

import (
	protojson "biblio/protocol/json"
	"encoding/json"
	"errors"
	"github.com/ZhangGuangxu/netbuffer"
	"hash/adler32"
)

var errInvalidMsgLength = errors.New("invalid message length")
var errChecksumNotMatch = errors.New("checksum not match")

type jsonCodec struct {
	tmpBuf *netbuffer.Buffer
}

func newJSONCodec() *jsonCodec {
	return &jsonCodec{
		tmpBuf: netbuffer.NewBuffer(),
	}
}

func (c *jsonCodec) Unpack(buf *netbuffer.Buffer, client *Client) error {
	for buf.ReadableBytes() >= headerByteCount+minDataLen {
		length := int(buf.PeekInt32())
		if length > maxDataLen || length < minDataLen {
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

func (c *jsonCodec) Pack(msg *message) ([]byte, error) {
	data, err := c.Encode(msg.proto)
	if err != nil {
		return nil, err
	}
	err = protojson.ProtoFactory.Release(msg.protoID, msg.proto)
	if err != nil {
		return nil, err
	}

	sumLen := protoIDByteCount + len(data)
	msgLen := sumLen + checkSumByteCount

	c.tmpBuf.RetrieveAll()

	c.tmpBuf.AppendInt16(msg.protoID)
	c.tmpBuf.Append(data)

	s := c.tmpBuf.PeekAsByteSlice(sumLen)
	v := adler32.Checksum(s)
	c.tmpBuf.AppendInt32(int32(v))

	c.tmpBuf.PrependInt32(int32(msgLen))

	return c.tmpBuf.PeekAllAsByteSlice(), nil
}

func (c *jsonCodec) Decode(protoID int16, data []byte) (interface{}, error) {
	proto, err := protojson.ProtoFactory.Require(protoID)
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
