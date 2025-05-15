//go:build !tinygo && !baremetal

package stub

import (
	"sync"
	"time"

	proto "github.com/ystepanoff/nrfcomm/protocol"
	"github.com/ystepanoff/nrfcomm/transport"
)

// Driver implements a mock radio driver for host-side testing
type Driver struct {
	mu      sync.Mutex
	rxQueue [][]byte
	txLog   [][]byte
}

// New creates a new stub driver for testing
func New() transport.RadioDriver { return &Driver{} }

func (d *Driver) StartHFCLK()                                                {}
func (d *Driver) Configure(address uint32, prefix byte, channel uint8) error { return nil }
func (d *Driver) SetChannel(channel uint8) error                             { return nil }

// Tx records transmitted packets for later inspection
func (d *Driver) Tx(data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	pkt := make([]byte, len(data))
	copy(pkt, data)
	d.txLog = append(d.txLog, pkt)
	return nil
}

// Rx returns queued packets or times out
func (d *Driver) Rx(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		d.mu.Lock()
		if len(d.rxQueue) > 0 {
			pkt := d.rxQueue[0]
			d.rxQueue = d.rxQueue[1:]
			d.mu.Unlock()
			out := make([]byte, len(pkt))
			copy(out, pkt)
			return out, nil
		}
		d.mu.Unlock()

		if time.Now().After(deadline) {
			return nil, proto.ErrTimeout
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// InjectRx queues packets to be returned by the next Rx() call
func (d *Driver) InjectRx(data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	pkt := make([]byte, len(data))
	copy(pkt, data)
	d.rxQueue = append(d.rxQueue, pkt)
}

// GetTxLog returns a copy of the transmitted packets log
func (d *Driver) GetTxLog() [][]byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([][]byte, len(d.txLog))
	for i, p := range d.txLog {
		cp := make([]byte, len(p))
		copy(cp, p)
		out[i] = cp
	}
	return out
}
