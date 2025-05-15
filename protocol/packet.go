package protocol

import "encoding/binary"

// Packet represents a frame of data transferred over the radio link.
// Layout: Length(1) | SenderID(4) | Type(1) | Reserved(2) | Payload(0-56)
// Length byte counts everything after itself.
// Total size max 64 bytes.

type DeviceID uint32

type Packet struct {
	Length   byte
	SenderID DeviceID
	Type     byte
	Reserved [2]byte
	Payload  []byte
}

// EncodePacket serialises a Packet into on-air bytes (length-prefixed).
func EncodePacket(p *Packet) []byte {
	if p == nil {
		return make([]byte, 0)
	}

	payloadLen := 0
	if p.Payload != nil {
		payloadLen = len(p.Payload)
	}

	bodyLen := min(headerWithoutLen+payloadLen, MaxPayloadSize-1)
	totalLen := min(bodyLen+1, MaxPacketSize)

	data := make([]byte, totalLen)
	data[0] = byte(bodyLen)
	binary.LittleEndian.PutUint32(data[1:5], uint32(p.SenderID))
	data[5] = p.Type
	data[6] = p.Reserved[0]
	data[7] = p.Reserved[1]

	usablePayloadLen := totalLen - PacketHeaderSize
	if usablePayloadLen > 0 && len(p.Payload) > 0 {
		copyLen := usablePayloadLen
		if copyLen > len(p.Payload) {
			copyLen = len(p.Payload)
		}
		copy(data[PacketHeaderSize:], p.Payload[:copyLen])
	}

	return data
}

func DecodePacket(data []byte) *Packet {
	if len(data) < PacketHeaderSize {
		return nil
	}

	bodyLen := int(data[0])
	if bodyLen == 0 || bodyLen > MaxPayloadSize-1 {
		return nil
	}

	totalLen := bodyLen + 1
	if totalLen > len(data) {
		return nil
	}

	if len(data) < 8 {
		return nil
	}

	p := &Packet{
		Length:   byte(bodyLen),
		SenderID: DeviceID(binary.LittleEndian.Uint32(data[1:5])),
		Type:     data[5],
		Reserved: [2]byte{data[6], data[7]},
	}

	payloadLen := bodyLen - headerWithoutLen
	if payloadLen > 0 {
		if PacketHeaderSize+payloadLen <= len(data) {
			p.Payload = make([]byte, payloadLen)
			copy(p.Payload, data[PacketHeaderSize:PacketHeaderSize+payloadLen])
		} else {
			p.Payload = make([]byte, 0)
		}
	} else {
		p.Payload = make([]byte, 0)
	}

	return p
}
