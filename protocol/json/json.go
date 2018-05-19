// Package json contains protocols in JSON form
package json

import (
	proto "biblio/protocol"
	"fmt"
	"sync"
)

// C2SLogin protocol login
type C2SLogin struct {
	UID   int64  `json:"uid"`
	Token string `json:"token"`
}

// ProtoFactory is a factory instance to create json instance.
var ProtoFactory = &factory{
	mapProtoID2Pool: map[int16]*sync.Pool{
		proto.C2SLoginID: &sync.Pool{New: func() interface{} { return &C2SLogin{} }},
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
