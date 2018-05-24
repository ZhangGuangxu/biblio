package main

import (
	proto "biblio/protocol"
	protojson "biblio/protocol/json"
)

var jsonCreater = &JSONCreater{}

// JSONCreater creates json proto instance.
type JSONCreater struct {
}

func (c *JSONCreater) createS2CAuth(passed bool) *message {
	v := &protojson.S2CAuth{
		Passed: passed,
	}
	protoID := proto.S2CAuthID
	proto, _ := protojson.ProtoFactory.RequireWithSourceProto(protoID, v)
	return &message{protoID, proto}
}

func (c *JSONCreater) createS2CClose(reason int8) *message {
	v := &protojson.S2CClose{
		Reason: reason,
	}
	protoID := proto.S2CAuthID
	proto, _ := protojson.ProtoFactory.RequireWithSourceProto(protoID, v)
	return &message{protoID, proto}
}
