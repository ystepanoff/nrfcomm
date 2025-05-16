package protocol

// Generic radio & protocol constants (platform independent). All higher layers should depend on this file.
const (
	// Frame sizing
	// Layout:
	//   Length (1 byte)  | SenderID (4) | Type (1) | Reserved (2)  | Payload (0-242) | CRC32 (4) | Terminal (1)
	// Length counts everything after the length byte, i.e., total Frame size minus 1.

	// Sizes of individual components
	LengthFieldSize   = 1
	SequenceFieldSize = 4
	CRCSize           = 4 // CRC32, little-endian
	TerminalSize      = 1

	// Header consists of: SenderID(4)+Type(1)+Seq(4) = 9 plus Length field = 10 bytes before payload
	FrameHeaderSize = LengthFieldSize + 4 + 1 + SequenceFieldSize // 10 bytes

	// Application-level payload allowance
	MaxPayloadSize = MaxFrameSize - FrameHeaderSize - CRCSize - TerminalSize

	// Total maximum Frame length on air (including length, CRC, Terminal)
	MaxFrameSize = 128

	// RF defaults (can be overridden per device)
	DefaultChannel = 7

	// Frame types
	FrameTypePairing   = 0x01
	FrameTypeData      = 0x02
	FrameTypeHeartbeat = 0x03
	FrameTypeAck       = 0x04

	// Timeouts / intervals (milliseconds)
	HeartbeatInterval = 5000
	PairingTimeout    = 30000
	DeviceTimeout     = 15000

	// internal helper (bytes in header after length byte)
	headerWithoutLen = FrameHeaderSize - LengthFieldSize

	// Terminal byte value appended to the end of every Frame
	FrameTerminal = 0x55
)
