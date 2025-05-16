// Package nrfcomm provides a façade to access the radio communication layer.
package nrfcomm

import (
	"github.com/ystepanoff/nrfcomm/protocol"
	"github.com/ystepanoff/nrfcomm/transport"
)

// The actual implementation is split into build-tag specific files:
// - constructors_nrf.go - for embedded platforms (//go:build tinygo || baremetal)
// - constructors_host.go - for development/testing (//go:build !tinygo && !baremetal)

// Re-export types for backward compatibility
type (
	DeviceID    = protocol.DeviceID
	DeviceType  = protocol.DeviceType
	Frame       = protocol.Frame
	Transmitter = transport.Transmitter
	Receiver    = transport.Receiver
)

// Error constants exposed in the public API
var (
	ErrInvalidPayload = protocol.ErrInvalidPayload
	ErrNotPaired      = protocol.ErrNotPaired
	ErrTimeout        = protocol.ErrTimeout
	ErrInvalidChannel = protocol.ErrInvalidChannel
)

// Constants exposed in the public API
const (
	DeviceTypeTransmitter = protocol.DeviceTypeTransmitter
	DeviceTypeReceiver    = protocol.DeviceTypeReceiver

	FrameTypePairing   = protocol.FrameTypePairing
	FrameTypeData      = protocol.FrameTypeData
	FrameTypeHeartbeat = protocol.FrameTypeHeartbeat
	FrameTypeAck       = protocol.FrameTypeAck
)
