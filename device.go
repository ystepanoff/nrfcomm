//go:build tinygo || baremetal

package nrfcomm

import (
	"encoding/binary"
	"time"
)

type DeviceID uint32
type DeviceType uint8

const (
	DeviceTypeTransmitter DeviceType = 1
	DeviceTypeReceiver    DeviceType = 2
)

// Device represents a radio device
type Device struct {
	ID         DeviceID
	Type       DeviceType
	Address    uint32
	Prefix     byte
	Channel    uint8
	LastSeen   int64
	IsPaired   bool
	PairingKey uint32
}

// Packet structure:
//
//	+--------+----------+---------+----------+-------------+
//	| Length | SenderID | Type    | Reserved |   Payload   |
//	+--------+----------+---------+----------+-------------+
//	| 1 byte | 4 bytes  | 1 byte  | 2 bytes  |  0-56 bytes |
//	+--------+----------+---------+----------+-------------+
//	Total header size: 8 bytes
//	Maximum payload size: 56 bytes (64 - 8 header bytes)
//	Maximum total packet size: 64 bytes
type Packet struct {
	Length   byte
	SenderID DeviceID
	Type     byte
	Reserved [2]byte
	Payload  []byte
}

func NewTransmitter(id DeviceID) *Device {
	return &Device{
		ID:       id,
		Type:     DeviceTypeTransmitter,
		Address:  uint32(id & 0xFFFFFFFF),
		Prefix:   byte((id >> 24) & 0xFF),
		Channel:  DefaultChannel,
		IsPaired: false,
	}
}

func NewReceiver(id DeviceID) *Device {
	return &Device{
		ID:       id,
		Type:     DeviceTypeReceiver,
		Address:  uint32(id & 0xFFFFFFFF),
		Prefix:   byte((id >> 24) & 0xFF),
		Channel:  DefaultChannel,
		IsPaired: false,
	}
}

// IsAlive checks if the device is still active based on last seen timestamp
func (d *Device) IsAlive() bool {
	if !d.IsPaired {
		return false
	}
	return (time.Now().UnixMilli() - d.LastSeen) < deviceTimeout
}

func (d *Device) UpdateLastSeen() {
	d.LastSeen = time.Now().UnixMilli()
}

// EncodePacket encodes a packet to bytes
func EncodePacket(p *Packet) []byte {
	totalLen := packetHeaderSize + len(p.Payload)
	if totalLen > MaxPayloadSize {
		totalLen = MaxPayloadSize
	}

	data := make([]byte, totalLen)
	data[0] = byte(totalLen)

	// Sender ID
	binary.LittleEndian.PutUint32(data[1:5], uint32(p.SenderID))

	// Packet type
	data[5] = p.Type

	// Reserved bytes
	data[6] = p.Reserved[0]
	data[7] = p.Reserved[1]

	// Payload
	payloadLen := totalLen - packetHeaderSize
	if payloadLen > 0 {
		copy(data[packetHeaderSize:], p.Payload[:payloadLen])
	}

	return data
}

// DecodePacket decodes bytes to a packet
func DecodePacket(data []byte) *Packet {
	if len(data) < packetHeaderSize {
		return nil
	}

	plen := data[0]
	if plen == 0 || plen > MaxPayloadSize {
		return nil
	}

	p := &Packet{
		Length:   plen,
		SenderID: DeviceID(binary.LittleEndian.Uint32(data[1:5])),
		Type:     data[5],
		Reserved: [2]byte{data[6], data[7]},
	}

	payloadLen := int(plen) - packetHeaderSize
	if payloadLen > 0 {
		p.Payload = make([]byte, payloadLen)
		copy(p.Payload, data[packetHeaderSize:packetHeaderSize+payloadLen])
	}

	return p
}
