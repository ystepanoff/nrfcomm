//go:build tinygo || baremetal

package nrf

import (
	proto "github.com/ystepanoff/nrfcomm/protocol"

	"device/nrf"
)

// StartHFCLK starts the high-frequency clock required by the radio.
func StartHFCLK() {
	nrf.CLOCK.EVENTS_HFCLKSTARTED.Set(0)
	nrf.CLOCK.TASKS_HFCLKSTART.Set(1)
	for nrf.CLOCK.EVENTS_HFCLKSTARTED.Get() == 0 {
	}
}

// ConfigureRadio sets up mode, power and addressing for the given channel.
func ConfigureRadio(address uint32, prefix byte, channel uint8) error {
	if channel > 125 {
		return proto.ErrInvalidChannel
	}

	nrf.RADIO.POWER.Set(1)
	nrf.RADIO.MODE.Set(nrf.RADIO_MODE_MODE_Nrf_1Mbit)
	nrf.RADIO.TXPOWER.Set(nrf.RADIO_TXPOWER_TXPOWER_0dBm)
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
		(proto.MaxFrameSize << nrf.RADIO_PCNF1_MAXLEN_Pos) |
			(0 << nrf.RADIO_PCNF1_STATLEN_Pos) |
			(3 << nrf.RADIO_PCNF1_BALEN_Pos) |
			(nrf.RADIO_PCNF1_ENDIAN_Little << nrf.RADIO_PCNF1_ENDIAN_Pos))

	nrf.RADIO.CRCCNF.Set(1)
	nrf.RADIO.CRCINIT.Set(0xFF)
	nrf.RADIO.CRCPOLY.Set(0x107)

	return nil
}
