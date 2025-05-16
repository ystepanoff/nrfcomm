package protocol

import (
	"encoding/binary"
	"hash/crc32"
)

// Frame represents a frame of data transferred over the radio link.
// Layout: Length(1) | SenderID(4) | Type(1) | Seq(4) | Reserved(2) | Payload(0-242) | CRC32(4) | Terminal(1)
// Length counts everything AFTER the length byte (so full Frame minus 1).
// Total size max 255 bytes.

type DeviceID uint32

type Frame struct {
	Length   byte
	SenderID DeviceID
	Type     byte
	Seq      uint32
	Payload  []byte
	CRC      uint32 // decoded Frames only; ignored by encoder
}

func EncodeFrame(p *Frame) []byte {
	if p == nil {
		return make([]byte, 0)
	}

	payloadLen := 0
	if p.Payload != nil {
		if len(p.Payload) > MaxPayloadSize {
			p.Payload = p.Payload[:MaxPayloadSize]
		}
		payloadLen = len(p.Payload)
	}

	bodyLen := headerWithoutLen + payloadLen + CRCSize + TerminalSize // bytes AFTER Length field
	if bodyLen > (MaxFrameSize - LengthFieldSize) {
		bodyLen = MaxFrameSize - LengthFieldSize
		if bodyLen >= headerWithoutLen+CRCSize+TerminalSize {
			payloadLen = bodyLen - headerWithoutLen - CRCSize - TerminalSize
		}
	}

	totalLen := int(LengthFieldSize) + bodyLen

	data := make([]byte, totalLen)
	data[0] = byte(bodyLen)
	binary.LittleEndian.PutUint32(data[1:5], uint32(p.SenderID))
	data[5] = p.Type
	binary.LittleEndian.PutUint32(data[6:10], p.Seq)

	if payloadLen > 0 {
		copy(data[FrameHeaderSize:], p.Payload[:payloadLen])
	}

	// Compute CRC32 of payload
	var crc uint32
	if payloadLen > 0 {
		crc = crc32.ChecksumIEEE(p.Payload[:payloadLen])
	} else {
		crc = 0
	}
	crcPos := FrameHeaderSize + payloadLen
	binary.LittleEndian.PutUint32(data[crcPos:crcPos+CRCSize], crc)

	// Terminal byte
	data[totalLen-1] = FrameTerminal

	p.Length = byte(bodyLen)

	return data
}

func DecodeFrame(data []byte) *Frame {
	// Must at least fit header + CRC + Terminal
	minLen := FrameHeaderSize + CRCSize + TerminalSize
	if len(data) < minLen {
		return nil
	}

	bodyLen := int(data[0])
	if bodyLen == 0 || (bodyLen+int(LengthFieldSize)) > len(data) {
		return nil
	}

	// Validate Terminal
	if data[int(LengthFieldSize)+bodyLen-1] != FrameTerminal {
		return nil
	}

	// Determine payload length
	payloadLen := bodyLen - headerWithoutLen - (CRCSize + TerminalSize)
	if payloadLen < 0 || payloadLen > MaxPayloadSize {
		return nil
	}

	payloadOffset := FrameHeaderSize
	crcOffset := payloadOffset + payloadLen

	if crcOffset+CRCSize > len(data) {
		return nil
	}

	recvCRC := binary.LittleEndian.Uint32(data[crcOffset : crcOffset+CRCSize])

	calcCRC := crc32.ChecksumIEEE(data[payloadOffset:crcOffset])
	if recvCRC != calcCRC {
		return nil
	}

	seqVal := binary.LittleEndian.Uint32(data[6:10])

	p := &Frame{
		Length:   byte(bodyLen),
		SenderID: DeviceID(binary.LittleEndian.Uint32(data[1:5])),
		Type:     data[5],
		Seq:      seqVal,
		CRC:      recvCRC,
	}

	if payloadLen > 0 {
		p.Payload = make([]byte, payloadLen)
		copy(p.Payload, data[payloadOffset:payloadOffset+payloadLen])
	} else {
		p.Payload = make([]byte, 0)
	}

	return p
}
