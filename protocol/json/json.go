// Package json contains protocols in JSON form
package json

import (
	proto "biblio/protocol"
	"fmt"
	"sync"
)

// C2SAuth protocol
type C2SAuth struct {
	UID   int64  `json:"uid"`
	Token string `json:"token"`
}

// S2CAuth protocol
type S2CAuth struct {
	Passed bool `json:"passed"`
}

// C2SHeartbeat protocol
type C2SHeartbeat struct {
}

// ProtoFactory is a factory instance to create json instance.
var ProtoFactory = &factory{
	mapProtoID2Pool: map[int16]*sync.Pool{
		proto.C2SAuthID:      &sync.Pool{New: func() interface{} { return &C2SAuth{} }},
		proto.S2CAuthID:      &sync.Pool{New: func() interface{} { return &S2CAuth{} }},
		proto.C2SHeartbeatID: &sync.Pool{New: func() interface{} { return &C2SHeartbeat{} }},
	},
}

type factory struct {
	mapProtoID2Pool map[int16]*sync.Pool
}

func (f *factory) Require(protoID int16) (interface{}, error) {
	if pool, ok := f.mapProtoID2Pool[protoID]; ok {
		return pool.Get(), nil
	}

	return nil, fmt.Errorf("Invalid proto id [%v]", protoID)
}

func (f *factory) Release(protoID int16, x interface{}) error {
	if pool, ok := f.mapProtoID2Pool[protoID]; ok {
		pool.Put(x)
		return nil
	}

	return fmt.Errorf("Invalid proto id [%v]", protoID)
}
