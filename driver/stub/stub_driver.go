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
	mu    sync.Mutex
	rxBuf ringBuffer
	txBuf ringBuffer
}

func New() transport.RadioDriver { return &Driver{} }

func (d *Driver) StartHFCLK()                                                {}
func (d *Driver) Configure(address uint32, prefix byte, channel uint8) error { return nil }
func (d *Driver) SetChannel(channel uint8) error                             { return nil }

func (d *Driver) Tx(data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	frame := make([]byte, len(data))
	copy(frame, data)
	d.txBuf.push(frame)
	return nil
}

func (d *Driver) Rx(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		d.mu.Lock()
		frame, ok := d.rxBuf.pop()
		d.mu.Unlock()
		if ok {
			out := make([]byte, len(frame))
			copy(out, frame)
			return out, nil
		}

		if time.Now().After(deadline) {
			return nil, proto.ErrTimeout
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func (d *Driver) InjectRx(data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	frame := make([]byte, len(data))
	copy(frame, data)
	d.rxBuf.push(frame)
}

func (d *Driver) GetTxLog() [][]byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.txBuf.snapshot()
}

const ringCapacity = 64

type ringBuffer struct {
	data       [ringCapacity][]byte
	head, tail int // head = next pop, tail = next push
	count      int
}

func (rb *ringBuffer) push(frame []byte) {
	if rb.count == ringCapacity {
		// Overwrite the oldest when buffer is full to keep memory bounded
		rb.data[rb.tail] = nil
		rb.head = (rb.head + 1) % ringCapacity
		rb.count--
	}
	rb.data[rb.tail] = frame
	rb.tail = (rb.tail + 1) % ringCapacity
	rb.count++
}

func (rb *ringBuffer) pop() ([]byte, bool) {
	if rb.count == 0 {
		return nil, false
	}
	frame := rb.data[rb.head]
	rb.data[rb.head] = nil
	rb.head = (rb.head + 1) % ringCapacity
	rb.count--
	return frame, true
}

func (rb *ringBuffer) snapshot() [][]byte {
	out := make([][]byte, rb.count)
	idx := 0
	i := rb.head
	for c := 0; c < rb.count; c++ {
		p := rb.data[i]
		cp := make([]byte, len(p))
		copy(cp, p)
		out[idx] = cp
		idx++
		i = (i + 1) % ringCapacity
	}
	return out
}
