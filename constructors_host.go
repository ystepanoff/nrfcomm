//go:build !tinygo && !baremetal

// This file is built only for non-embedded targets (host-based testing).
package nrfcomm

import (
	"github.com/ystepanoff/nrfcomm/driver/stub"
	"github.com/ystepanoff/nrfcomm/protocol"
	"github.com/ystepanoff/nrfcomm/transport"
)

func NewTransmitter(id protocol.DeviceID) *transport.Transmitter {
	return transport.NewTransmitterWithDriver(id, stub.New())
}

func NewReceiver(id protocol.DeviceID) *transport.Receiver {
	return transport.NewReceiverWithDriver(id, stub.New())
}
