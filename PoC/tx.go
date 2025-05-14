//go:build tinygo || baremetal

package main

import (
	"encoding/binary"
	"time"
	"unsafe"

	"device/nrf"
)

const maxPayload = 64

var buf [1 + maxPayload]byte
var counter uint32

func main() {
	startHFCLK()
	configRadio()

	for {
		plen := byte((counter % 64) + 1)

		binary.LittleEndian.PutUint32(buf[1:], counter)

		buf[0] = plen // length byte
		nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&buf[0]))))

		// clear previous EVENT flags
		nrf.RADIO.EVENTS_READY.Set(0)
		nrf.RADIO.EVENTS_END.Set(0)

		// enable TX → READY
		nrf.RADIO.TASKS_TXEN.Set(1)
		for nrf.RADIO.EVENTS_READY.Get() == 0 {
		}

		// start one packet → END
		nrf.RADIO.TASKS_START.Set(1)
		for nrf.RADIO.EVENTS_END.Get() == 0 {
		}

		// disable the radio before the next loop
		nrf.RADIO.TASKS_DISABLE.Set(1)
		for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}

func startHFCLK() {
	nrf.CLOCK.EVENTS_HFCLKSTARTED.Set(0)
	nrf.CLOCK.TASKS_HFCLKSTART.Set(1)
	for nrf.CLOCK.EVENTS_HFCLKSTARTED.Get() == 0 {
	}
}

func configRadio() {
	nrf.RADIO.POWER.Set(1) // make sure the peripheral is on

	nrf.RADIO.MODE.Set(nrf.RADIO_MODE_MODE_Nrf_1Mbit)
	nrf.RADIO.TXPOWER.Set(nrf.RADIO_TXPOWER_TXPOWER_0dBm)
	nrf.RADIO.FREQUENCY.Set(7) // 2407 MHz

	/* 5-byte address: E7 E7 E7 E7 E7 */
	nrf.RADIO.BASE0.Set(0xE7E7E7E7)
	nrf.RADIO.PREFIX0.Set(0xE7)
	nrf.RADIO.TXADDRESS.Set(0)   // pipe 0
	nrf.RADIO.RXADDRESSES.Set(1) // listen on pipe 0 (needed for ACK later)

	nrf.RADIO.PCNF0.Set(
		(8 << nrf.RADIO_PCNF0_LFLEN_Pos) | // 8-bit length
			(0 << nrf.RADIO_PCNF0_S0LEN_Pos) | // no S0
			(0 << nrf.RADIO_PCNF0_S1LEN_Pos)) // no S1

	nrf.RADIO.PCNF1.Set(
		(64 << nrf.RADIO_PCNF1_MAXLEN_Pos) | // ≤64 B after length byte
			(0 << nrf.RADIO_PCNF1_STATLEN_Pos) | // variable
			(3 << nrf.RADIO_PCNF1_BALEN_Pos) | // 5-B base+prefix
			(nrf.RADIO_PCNF1_ENDIAN_Little << nrf.RADIO_PCNF1_ENDIAN_Pos))

	/* 1-byte CRC */
	nrf.RADIO.CRCCNF.Set(1)
	nrf.RADIO.CRCINIT.Set(0xFF)
	nrf.RADIO.CRCPOLY.Set(0x107)
}
