//go:build tinygo || baremetal

package nrf

import (
	"time"
	"unsafe"

	proto "github.com/ystepanoff/nrfcomm/protocol"
	"github.com/ystepanoff/nrfcomm/transport"

	"device/nrf"
)

// Driver provides a RadioDriver backed by the real NRF peripheral registers.
// It keeps an internal buffer for Frame TX/RX operations.
type Driver struct {
	buffer [proto.MaxFrameSize + 1]byte
}

func New() transport.RadioDriver { return &Driver{} }

func (d *Driver) StartHFCLK() { StartHFCLK() }

func (d *Driver) Configure(address uint32, prefix byte, channel uint8) error {
	return ConfigureRadio(address, prefix, channel)
}

func (d *Driver) SetChannel(channel uint8) error {
	if channel > 125 {
		return proto.ErrInvalidChannel
	}
	nrf.RADIO.FREQUENCY.Set(uint32(channel))
	return nil
}

func (d *Driver) Tx(data []byte) error {
	if len(data) > proto.MaxFrameSize {
		return proto.ErrInvalidPayload
	}
	copy(d.buffer[:], data)
	nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&d.buffer[0]))))
	nrf.RADIO.EVENTS_READY.Set(0)
	nrf.RADIO.EVENTS_END.Set(0)
	nrf.RADIO.TASKS_TXEN.Set(1)
	for nrf.RADIO.EVENTS_READY.Get() == 0 {
	}
	nrf.RADIO.TASKS_START.Set(1)
	for nrf.RADIO.EVENTS_END.Get() == 0 {
	}
	nrf.RADIO.TASKS_DISABLE.Set(1)
	for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
	}
	return nil
}

func (d *Driver) Rx(timeout time.Duration) ([]byte, error) {
	nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&d.buffer[0]))))
	nrf.RADIO.EVENTS_READY.Set(0)
	nrf.RADIO.EVENTS_END.Set(0)
	nrf.RADIO.TASKS_RXEN.Set(1)
	for nrf.RADIO.EVENTS_READY.Get() == 0 {
	}
	nrf.RADIO.TASKS_START.Set(1)
	start := time.Now()
	for nrf.RADIO.EVENTS_END.Get() == 0 {
		if time.Since(start) > timeout {
			nrf.RADIO.TASKS_DISABLE.Set(1)
			for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
			}
			return nil, proto.ErrTimeout
		}
	}
	nrf.RADIO.TASKS_DISABLE.Set(1)
	for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
	}
	frameLen := int(d.buffer[0]) + proto.LengthFieldSize
	if frameLen > proto.MaxFrameSize {
		frameLen = proto.MaxFrameSize
	}
	out := make([]byte, frameLen)
	copy(out, d.buffer[:frameLen])
	return out, nil
}
