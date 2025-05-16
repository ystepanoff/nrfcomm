package protocol

import "time"

// Device represents either a transmitter or receiver device in memory.
// It stores pairing key and last-seen timestamp used by higher layers.

type DeviceType uint8

const (
	DeviceTypeTransmitter DeviceType = 1
	DeviceTypeReceiver    DeviceType = 2
)

type Device struct {
	ID      DeviceID
	Address uint32
	Prefix  byte
	Channel uint8

	PairingKey uint32
	IsPaired   bool
	LastSeen   int64 // unix milli
}

func newDevice(id DeviceID) *Device {
	return &Device{
		ID:       id,
		Address:  0xE7E7E7E7,
		Prefix:   0xE7,
		Channel:  DefaultChannel,
		LastSeen: time.Now().UnixMilli(),
	}
}

func NewTransmitter(id DeviceID) *Device { return newDevice(id) }

func NewReceiver(id DeviceID) *Device { return newDevice(id) }

func (d *Device) UpdateLastSeen() { d.LastSeen = time.Now().UnixMilli() }

func (d *Device) IsAlive() bool { return (time.Now().UnixMilli() - d.LastSeen) < DeviceTimeout }
