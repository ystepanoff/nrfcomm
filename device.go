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

// Device represents either a transmitter or receiver device in memory.
type Device struct {
	ID      DeviceID
	Address uint32
	Prefix  byte
	Channel uint8

	PairingKey uint32
	IsPaired   bool
	LastSeen   int64 // unix milli
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

const headerWithoutLen = packetHeaderSize - 1

func EncodePacket(p *Packet) []byte {
	bodyLen := headerWithoutLen + len(p.Payload) // bytes after length byte
	if bodyLen > MaxPayloadSize-1 {
		bodyLen = MaxPayloadSize - 1
	}

	totalLen := bodyLen + 1 // include length byte
	data := make([]byte, totalLen)
	data[0] = byte(bodyLen)

	// Sender ID (4 bytes)
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

func DecodePacket(data []byte) *Packet {
	if len(data) < packetHeaderSize {
		return nil
	}

	bodyLen := int(data[0])
	if bodyLen == 0 || bodyLen > MaxPayloadSize-1 {
		return nil
	}

	if bodyLen+1 > len(data) {
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
		p.Payload = make([]byte, payloadLen)
		copy(p.Payload, data[packetHeaderSize:packetHeaderSize+payloadLen])
	}

	return p
}

func newDevice(id DeviceID) *Device {
	return &Device{
		ID:       id,
		Address:  0xE7E7E7E7,
		Prefix:   0xE7,
		Channel:  80,
		LastSeen: time.Now().UnixMilli(),
	}
}

func NewTransmitter(id DeviceID) *Device {
	return newDevice(id)
}

func NewReceiver(id DeviceID) *Device {
	return newDevice(id)
}

func (d *Device) UpdateLastSeen() {
	d.LastSeen = time.Now().UnixMilli()
}

func (d *Device) IsAlive() bool {
	return (time.Now().UnixMilli() - d.LastSeen) < deviceTimeout
}
