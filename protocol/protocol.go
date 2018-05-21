// Package protocol defines protocol ids and some other stuff.
package protocol

// ProtoFactory defines a interface with methods to
// require and release proto instances.
type ProtoFactory interface {
	Require(protoID int16) (interface{}, error)
	Release(protoID int16, x interface{}) error
}

// C2S protocol
const (
	C2SAuthID      int16 = 100
	C2SHeartbeatID int16 = 101
)

// S2C protocol
const (
	S2CAuthID int16 = 500
)
