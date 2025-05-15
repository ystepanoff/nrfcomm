//go:build tinygo || baremetal

// This file is built only for embedded targets (using real radio hardware).
package nrfcomm

import (
	"github.com/ystepanoff/nrfcomm/driver/nrf"
	"github.com/ystepanoff/nrfcomm/protocol"
	"github.com/ystepanoff/nrfcomm/transport"
)

func NewTransmitter(id protocol.DeviceID) *transport.Transmitter {
	return transport.NewTransmitterWithDriver(id, nrf.New())
}

func NewReceiver(id protocol.DeviceID) *transport.Receiver {
	return transport.NewReceiverWithDriver(id, nrf.New())
}
