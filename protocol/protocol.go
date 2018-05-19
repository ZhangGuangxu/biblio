// Package protocol defines protocol ids and some other stuff.
package protocol

// ProtoFactory defines a interface with methods to
// require and release proto instances.
type ProtoFactory interface {
	Require(protoID int16) (interface{}, error)
	Release(protoID int16, x interface{}) error
}

const (
	C2SLoginID int16 = 200
)

const (
	S2CLoginID int16 = 200
)
