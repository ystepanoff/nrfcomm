package protocol

// Generic radio & protocol constants (platform independent). All higher layers should depend on this file.
const (
	// Packet sizing
	MaxPacketSize    = 64 // 1-byte length + 63 bytes rest (header+payload)
	PacketHeaderSize = 8  // 1 length + 4 senderID + 1 type + 2 reserved
	MaxPayloadSize   = MaxPacketSize - PacketHeaderSize

	// RF defaults (can be overridden per device)
	DefaultChannel = 80

	// Packet types
	PacketTypePairing   = 0x01
	PacketTypeData      = 0x02
	PacketTypeHeartbeat = 0x03
	PacketTypeAck       = 0x04

	// Timeouts / intervals (milliseconds)
	HeartbeatInterval = 5000
	PairingTimeout    = 10000
	DeviceTimeout     = 15000

	// internal helper (bytes in header after length byte)
	headerWithoutLen = PacketHeaderSize - 1
)
