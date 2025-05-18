package protocol

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestFrameEncoding(t *testing.T) {
	tests := []struct {
		name        string
		frame       *Frame
		wantMinSize int
		wantMaxSize int
	}{
		{
			name: "empty payload",
			frame: &Frame{
				SenderID: 0xCAFE,
				Type:     FrameTypeData,
				Seq:      42,
				Payload:  []byte{},
			},
			wantMinSize: FrameHeaderSize + CRCSize + TerminalSize,
			wantMaxSize: FrameHeaderSize + CRCSize + TerminalSize,
		},
		{
			name: "small payload",
			frame: &Frame{
				SenderID: 0xBEEF,
				Type:     FrameTypeData,
				Seq:      123,
				Payload:  []byte{1, 2, 3, 4, 5},
			},
			wantMinSize: FrameHeaderSize + 5 + CRCSize + TerminalSize,
			wantMaxSize: FrameHeaderSize + 5 + CRCSize + TerminalSize,
		},
		{
			name: "maximum payload",
			frame: &Frame{
				SenderID: 0xDEAD,
				Type:     FrameTypeData,
				Seq:      255,
				Payload:  bytes.Repeat([]byte{0xAA}, MaxPayloadSize),
			},
			wantMinSize: FrameHeaderSize + MaxPayloadSize + CRCSize + TerminalSize,
			wantMaxSize: MaxFrameSize,
		},
		{
			name: "too large payload gets truncated",
			frame: &Frame{
				SenderID: 0xDEAD,
				Type:     FrameTypeData,
				Seq:      255,
				Payload:  bytes.Repeat([]byte{0xAA}, MaxPayloadSize+50),
			},
			wantMinSize: FrameHeaderSize + MaxPayloadSize + CRCSize + TerminalSize,
			wantMaxSize: MaxFrameSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeFrame(tt.frame)

			// Verify size constraints
			if len(encoded) < tt.wantMinSize {
				t.Errorf("EncodeFrame() size = %v, want at least %v", len(encoded), tt.wantMinSize)
			}
			if len(encoded) > tt.wantMaxSize {
				t.Errorf("EncodeFrame() size = %v, exceeded max %v", len(encoded), tt.wantMaxSize)
			}

			// Verify structure
			if encoded[0] != byte(len(encoded)-1) { // Length byte should be total-1
				t.Errorf("Length byte = %v, want %v", encoded[0], len(encoded)-1)
			}

			// Verify SenderID
			gotSenderID := DeviceID(binary.LittleEndian.Uint32(encoded[1:5]))
			if gotSenderID != tt.frame.SenderID {
				t.Errorf("SenderID = %v, want %v", gotSenderID, tt.frame.SenderID)
			}

			// Verify Type
			if encoded[5] != tt.frame.Type {
				t.Errorf("Type = %v, want %v", encoded[5], tt.frame.Type)
			}

			// Verify Seq
			gotSeq := binary.LittleEndian.Uint32(encoded[6:10])
			if gotSeq != tt.frame.Seq {
				t.Errorf("Seq = %v, want %v", gotSeq, tt.frame.Seq)
			}

			// Check terminal byte
			if encoded[len(encoded)-1] != FrameTerminal {
				t.Errorf("Terminal byte = %v, want %v", encoded[len(encoded)-1], FrameTerminal)
			}

			// Check CRC is present
			payloadLen := len(encoded) - (FrameHeaderSize + CRCSize + TerminalSize)
			if payloadLen > 0 {
				crcPos := FrameHeaderSize + payloadLen
				gotCRC := binary.LittleEndian.Uint32(encoded[crcPos : crcPos+CRCSize])

				var expectedCRC uint32
				if payloadLen > 0 {
					expectedCRC = crc32.ChecksumIEEE(encoded[FrameHeaderSize:crcPos])
				}

				if gotCRC != expectedCRC {
					t.Errorf("CRC = %v, want %v", gotCRC, expectedCRC)
				}
			}
		})
	}
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		frame   *Frame
		wantErr bool
	}{
		{
			name: "empty payload",
			frame: &Frame{
				SenderID: 0xCAFE,
				Type:     FrameTypeData,
				Seq:      42,
				Payload:  []byte{},
			},
		},
		{
			name: "small payload",
			frame: &Frame{
				SenderID: 0xBEEF,
				Type:     FrameTypeData,
				Seq:      123,
				Payload:  []byte{1, 2, 3, 4, 5},
			},
		},
		{
			name: "maximum payload",
			frame: &Frame{
				SenderID: 0xDEAD,
				Type:     FrameTypeData,
				Seq:      255,
				Payload:  bytes.Repeat([]byte{0xAA}, MaxPayloadSize),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeFrame(tt.frame)
			decoded := DecodeFrame(encoded)

			if decoded == nil && !tt.wantErr {
				t.Fatal("DecodeFrame() returned nil, want successful decode")
			}
			if decoded != nil && tt.wantErr {
				t.Fatal("DecodeFrame() returned frame, want error/nil")
			}

			if tt.wantErr {
				return
			}

			// Compare fields
			if decoded.SenderID != tt.frame.SenderID {
				t.Errorf("SenderID = %v, want %v", decoded.SenderID, tt.frame.SenderID)
			}
			if decoded.Type != tt.frame.Type {
				t.Errorf("Type = %v, want %v", decoded.Type, tt.frame.Type)
			}
			if decoded.Seq != tt.frame.Seq {
				t.Errorf("Seq = %v, want %v", decoded.Seq, tt.frame.Seq)
			}

			// Compare payloads
			if len(decoded.Payload) != len(tt.frame.Payload) {
				t.Errorf("Payload length = %v, want %v", len(decoded.Payload), len(tt.frame.Payload))
			} else if len(decoded.Payload) > 0 && !bytes.Equal(decoded.Payload, tt.frame.Payload) {
				t.Errorf("Payload mismatch")
			}
		})
	}
}

func TestDecodeInvalidFrames(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "nil data",
			data: nil,
		},
		{
			name: "too short",
			data: []byte{0x01, 0x02},
		},
		{
			name: "bad length byte",
			data: append(
				[]byte{
					0xFF,                   // Length (impossibly large)
					0xEF, 0xBE, 0x00, 0x00, // SenderID
					0x01,                   // Type
					0x01, 0x00, 0x00, 0x00, // Seq
				},
				bytes.Repeat([]byte{0x00}, 10)..., // Some payload + junk
			),
		},
		{
			name: "wrong terminal byte",
			data: func() []byte {
				frame := &Frame{
					SenderID: 0xBEEF,
					Type:     FrameTypeData,
					Seq:      1,
					Payload:  []byte{1, 2, 3},
				}
				data := EncodeFrame(frame)
				data[len(data)-1] = 0xAA // Replace terminal byte
				return data
			}(),
		},
		{
			name: "corrupt CRC",
			data: func() []byte {
				frame := &Frame{
					SenderID: 0xBEEF,
					Type:     FrameTypeData,
					Seq:      1,
					Payload:  []byte{1, 2, 3},
				}
				data := EncodeFrame(frame)
				crcPos := FrameHeaderSize + len(frame.Payload)
				data[crcPos] ^= 0xFF // Flip bits in CRC
				return data
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeFrame(tt.data)
			if decoded != nil {
				t.Errorf("DecodeFrame() = %v, want nil for invalid frame", decoded)
			}
		})
	}
}

func TestFrameSizeLimit(t *testing.T) {
	// Create frame with oversized payload
	frame := &Frame{
		SenderID: 0xBEEF,
		Type:     FrameTypeData,
		Seq:      1,
		Payload:  bytes.Repeat([]byte{0xAA}, MaxPayloadSize*2),
	}

	// Encode
	encoded := EncodeFrame(frame)

	// Verify total size doesn't exceed limit
	if len(encoded) > MaxFrameSize {
		t.Errorf("EncodeFrame() size = %v, want <= %v", len(encoded), MaxFrameSize)
	}

	// Verify payload was truncated
	decoded := DecodeFrame(encoded)
	if decoded == nil {
		t.Fatal("DecodeFrame() returned nil, expected valid frame")
	}

	if len(decoded.Payload) > MaxPayloadSize {
		t.Errorf("Decoded payload size = %v, want <= %v", len(decoded.Payload), MaxPayloadSize)
	}
}
