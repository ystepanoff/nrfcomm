//go:build tinygo || baremetal

package nrfcomm

import (
	"errors"

	"device/nrf"
)

const (
	MaxPacketSize  = 64                               // Total packet size including header (8 bytes) and payload (up to 56 bytes)
	MaxPayloadSize = MaxPacketSize - packetHeaderSize // Maximum payload size is 56 bytes
	DefaultChannel = 7
	DefaultTxPower = nrf.RADIO_TXPOWER_TXPOWER_0dBm
	DefaultMode    = nrf.RADIO_MODE_MODE_Nrf_1Mbit

	packetHeaderSize = 8 // 1 byte length + 4 bytes device ID + 1 byte type + 2 bytes reserved

	packetTypePairing   = 0x01
	packetTypeData      = 0x02
	packetTypeHeartbeat = 0x03
	packetTypeAck       = 0x04

	// Timeouts in milliseconds
	heartbeatInterval = 5000
	pairingTimeout    = 10000
	deviceTimeout     = 15000 // Device considered dead after this timeout
)

var (
	ErrInvalidPayload = errors.New("invalid payload size")
	ErrNotPaired      = errors.New("device not paired")
	ErrTimeout        = errors.New("operation timed out")
	ErrInvalidChannel = errors.New("invalid channel (valid range: 0-125)")
)

// StartHFCLK starts the high-frequency clock
func StartHFCLK() {
	nrf.CLOCK.EVENTS_HFCLKSTARTED.Set(0)
	nrf.CLOCK.TASKS_HFCLKSTART.Set(1)
	for nrf.CLOCK.EVENTS_HFCLKSTARTED.Get() == 0 {
	}
}

func ConfigureRadio(address uint32, prefix byte, channel uint8) error {
	if channel > 125 {
		return ErrInvalidChannel
	}

	nrf.RADIO.POWER.Set(1)

	nrf.RADIO.MODE.Set(DefaultMode)
	nrf.RADIO.TXPOWER.Set(DefaultTxPower)
	nrf.RADIO.FREQUENCY.Set(uint32(channel))

	nrf.RADIO.BASE0.Set(address)
	nrf.RADIO.PREFIX0.Set(uint32(prefix))
	nrf.RADIO.TXADDRESS.Set(0)
	nrf.RADIO.RXADDRESSES.Set(1)

	nrf.RADIO.PCNF0.Set(
		(8 << nrf.RADIO_PCNF0_LFLEN_Pos) |
			(0 << nrf.RADIO_PCNF0_S0LEN_Pos) |
			(0 << nrf.RADIO_PCNF0_S1LEN_Pos))

	nrf.RADIO.PCNF1.Set(
		(MaxPayloadSize << nrf.RADIO_PCNF1_MAXLEN_Pos) |
			(0 << nrf.RADIO_PCNF1_STATLEN_Pos) |
			(3 << nrf.RADIO_PCNF1_BALEN_Pos) |
			(nrf.RADIO_PCNF1_ENDIAN_Little << nrf.RADIO_PCNF1_ENDIAN_Pos))

	nrf.RADIO.CRCCNF.Set(1)
	nrf.RADIO.CRCINIT.Set(0xFF)
	nrf.RADIO.CRCPOLY.Set(0x107)

	return nil
}
