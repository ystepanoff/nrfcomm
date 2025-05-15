package transport

import "time"

// RadioDriver is the interface that wraps the basic radio operations.
type RadioDriver interface {
	StartHFCLK()
	Configure(address uint32, prefix byte, channel uint8) error
	SetChannel(channel uint8) error
	Tx(data []byte) error
	Rx(timeout time.Duration) ([]byte, error)
}
