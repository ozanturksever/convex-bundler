package adminkey

import (
	"encoding/binary"
)

// adminIdentityType represents the type of admin identity
type adminIdentityType int

const (
	// adminIdentityMember represents a member identity with a member ID
	adminIdentityMember adminIdentityType = iota
	// adminIdentitySystem represents a system identity
	adminIdentitySystem
)

// adminKeyProto represents the admin key protobuf message.
// This is manually encoded to match the Rust prost encoding.
//
// Message definition (from convex-backend):
//
//	message AdminKeyProto {
//	  optional string instance_name = 1;
//	  uint64 issued_s = 2;
//	  oneof identity {
//	    uint64 member_id = 3;
//	    google.protobuf.Empty system = 4;
//	  }
//	  bool is_read_only = 5;
//	}
type adminKeyProto struct {
	instanceName *string // tag 1, optional
	issuedS      uint64  // tag 2
	identityType adminIdentityType
	memberID     uint64 // tag 3, only if identityType == adminIdentityMember
	isReadOnly   bool   // tag 5
}

// encode encodes the adminKeyProto to protobuf wire format
func (p *adminKeyProto) encode() []byte {
	var buf []byte

	// Field 1: instance_name (optional string)
	if p.instanceName != nil {
		buf = appendTag(buf, 1, wireTypeLengthDelimited)
		buf = appendString(buf, *p.instanceName)
	}

	// Field 2: issued_s (uint64)
	if p.issuedS != 0 {
		buf = appendTag(buf, 2, wireTypeVarint)
		buf = appendVarint(buf, p.issuedS)
	}

	// Field 3 or 4: identity (oneof)
	switch p.identityType {
	case adminIdentityMember:
		// Field 3: member_id (uint64)
		buf = appendTag(buf, 3, wireTypeVarint)
		buf = appendVarint(buf, p.memberID)
	case adminIdentitySystem:
		// Field 4: system (google.protobuf.Empty - encoded as empty message)
		buf = appendTag(buf, 4, wireTypeLengthDelimited)
		buf = appendVarint(buf, 0) // empty message has length 0
	}

	// Field 5: is_read_only (bool)
	if p.isReadOnly {
		buf = appendTag(buf, 5, wireTypeVarint)
		buf = appendVarint(buf, 1)
	}

	return buf
}

// Protobuf wire types
const (
	wireTypeVarint          = 0
	wireTypeLengthDelimited = 2
)

// appendTag appends a protobuf tag to the buffer
func appendTag(buf []byte, fieldNumber int, wireType int) []byte {
	tag := uint64(fieldNumber<<3 | wireType)
	return appendVarint(buf, tag)
}

// appendVarint appends a varint-encoded uint64 to the buffer
func appendVarint(buf []byte, v uint64) []byte {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], v)
	return append(buf, tmp[:n]...)
}

// appendString appends a length-delimited string to the buffer
func appendString(buf []byte, s string) []byte {
	buf = appendVarint(buf, uint64(len(s)))
	return append(buf, s...)
}
