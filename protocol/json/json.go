// Package json contains protocols in JSON form
package json

import (
	proto "biblio/protocol"
	"errors"
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

// S2CClose protocol
type S2CClose struct {
	Reason int8 `json:"reason"`
}

type protoSetFunc func(interface{}, interface{}) error

var errS2CAuthSrcTypeWrong = errors.New("S2CAuth src type wrong")
var errS2CAuthDstTypeWrong = errors.New("S2CAuth dst type wrong")
var errS2CCloseSrcTypeWrong = errors.New("S2CClose src type wrong")
var errS2CCloseDstTypeWrong = errors.New("S2CClose dst type wrong")

// ProtoFactory is a factory instance to create json instance.
var ProtoFactory = &factory{
	mapProtoID2Pool: map[int16]*sync.Pool{
		proto.C2SAuthID:      &sync.Pool{New: func() interface{} { return &C2SAuth{} }},
		proto.S2CAuthID:      &sync.Pool{New: func() interface{} { return &S2CAuth{} }},
		proto.C2SHeartbeatID: &sync.Pool{New: func() interface{} { return &C2SHeartbeat{} }},
		proto.S2CCloseID:     &sync.Pool{New: func() interface{} { return &S2CClose{} }},
	},
	protoSetter: map[int16]protoSetFunc{
		proto.S2CAuthID: func(dst interface{}, src interface{}) error {
			if d, ok := dst.(*S2CAuth); ok {
				if s, ok := src.(*S2CAuth); ok {
					*d = *s
					return nil
				}
				return errS2CAuthSrcTypeWrong
			}
			return errS2CAuthDstTypeWrong
		},
		proto.S2CCloseID: func(dst interface{}, src interface{}) error {
			if d, ok := dst.(*S2CClose); ok {
				if s, ok := src.(*S2CClose); ok {
					*d = *s
					return nil
				}
				return errS2CCloseSrcTypeWrong
			}
			return errS2CCloseDstTypeWrong
		},
	},
}

type factory struct {
	mapProtoID2Pool map[int16]*sync.Pool
	protoSetter     map[int16]protoSetFunc
}

func (f *factory) Require(protoID int16) (interface{}, error) {
	if pool, ok := f.mapProtoID2Pool[protoID]; ok {
		return pool.Get(), nil
	}

	return nil, fmt.Errorf("protoID[%v] has no pool", protoID)
}

func (f *factory) RequireWithSourceProto(protoID int16, src interface{}) (interface{}, error) {
	if pool, ok := f.mapProtoID2Pool[protoID]; ok {
		dst := pool.Get()
		if fn, ok := f.protoSetter[protoID]; ok {
			if err := fn(dst, src); err != nil {
				return nil, err
			}
			return dst, nil
		}
		return nil, fmt.Errorf("protoID[%v] has no setter", protoID)
	}

	return nil, fmt.Errorf("protoID[%v] has no pool", protoID)
}

func (f *factory) Release(protoID int16, x interface{}) error {
	if pool, ok := f.mapProtoID2Pool[protoID]; ok {
		pool.Put(x)
		return nil
	}

	return fmt.Errorf("protoID[%v] has no pool", protoID)
}
