package transport

import (
	"bytes"
	"sync"
	"testing"
	"time"

	proto "github.com/ystepanoff/nrfcomm/protocol"
)

// MockDriver implements the RadioDriver interface for testing
type MockDriver struct {
	mutex  sync.Mutex
	txLog  [][]byte
	rxData [][]byte
}

func NewMockDriver() *MockDriver {
	return &MockDriver{
		txLog:  make([][]byte, 0),
		rxData: make([][]byte, 0),
	}
}

func (d *MockDriver) StartHFCLK() {}

func (d *MockDriver) Configure(address uint32, prefix byte, channel uint8) error {
	return nil
}

func (d *MockDriver) SetChannel(channel uint8) error {
	return nil
}

func (d *MockDriver) Tx(data []byte) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Make a copy to avoid data races
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	d.txLog = append(d.txLog, dataCopy)
	return nil
}

func (d *MockDriver) Rx(timeout time.Duration) ([]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if len(d.rxData) == 0 {
		return nil, proto.ErrTimeout
	}

	data := d.rxData[0]
	d.rxData = d.rxData[1:]

	// Make a copy to avoid data races
	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}

// Test helper methods
func (d *MockDriver) GetTxLog() [][]byte {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Return a copy to avoid data races
	result := make([][]byte, len(d.txLog))
	for i, data := range d.txLog {
		result[i] = make([]byte, len(data))
		copy(result[i], data)
	}

	return result
}

func (d *MockDriver) ClearTxLog() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.txLog = d.txLog[:0]
}

func (d *MockDriver) InjectRx(data []byte) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Make a copy to avoid data races
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	d.rxData = append(d.rxData, dataCopy)
}

// Create a bidirectional channel between drivers
func ConnectDrivers(a, b *MockDriver) {
	go func() {
		for {
			// Forward a's transmissions to b
			txLog := a.GetTxLog()
			for _, data := range txLog {
				b.InjectRx(data)
			}
			a.ClearTxLog()

			// Forward b's transmissions to a
			txLog = b.GetTxLog()
			for _, data := range txLog {
				a.InjectRx(data)
			}
			b.ClearTxLog()

			time.Sleep(time.Millisecond)
		}
	}()
}

func TestTransmitter_SendFrame(t *testing.T) {
	// Create mock driver
	driver := NewMockDriver()

	// Create transmitter
	tx := NewTransmitterWithDriver(0xCAFE, driver)

	// Mark as paired to allow sending
	tx.device.IsPaired = true

	// Send test packets of different types
	tests := []struct {
		name      string
		frameType byte
		payload   []byte
	}{
		{
			name:      "empty payload",
			frameType: proto.FrameTypeData,
			payload:   []byte{},
		},
		{
			name:      "small payload",
			frameType: proto.FrameTypeData,
			payload:   []byte{1, 2, 3, 4, 5},
		},
		{
			name:      "heartbeat",
			frameType: proto.FrameTypeHeartbeat,
			payload:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous transmissions
			driver.ClearTxLog()

			// Send packet
			err := tx.SendFrame(tt.frameType, tt.payload)
			if err != nil {
				t.Fatalf("SendFrame() error = %v", err)
			}

			// Check if transmitter sent something
			txLog := driver.GetTxLog()
			if len(txLog) == 0 {
				t.Fatal("No packet transmitted")
			}

			// Decode the transmitted frame
			sent := proto.DecodeFrame(txLog[0])
			if sent == nil {
				t.Fatal("Transmitted invalid frame")
			}

			// Verify frame properties
			if sent.Type != tt.frameType {
				t.Errorf("Frame.Type = %v, want %v", sent.Type, tt.frameType)
			}

			if sent.SenderID != 0xCAFE {
				t.Errorf("Frame.SenderID = %v, want %v", sent.SenderID, 0xCAFE)
			}

			// Verify payload if expected
			if tt.payload != nil {
				if !bytes.Equal(sent.Payload, tt.payload) {
					t.Errorf("Frame.Payload = %v, want %v", sent.Payload, tt.payload)
				}
			}
		})
	}
}

func TestTransmitter_NotPaired(t *testing.T) {
	// Create mock driver
	driver := NewMockDriver()

	// Create transmitter (not paired)
	tx := NewTransmitterWithDriver(0xCAFE, driver)

	// Try to send data before pairing
	err := tx.SendData([]byte{1, 2, 3})
	if err != proto.ErrNotPaired {
		t.Errorf("SendData() error = %v, want %v", err, proto.ErrNotPaired)
	}

	// Try to send heartbeat before pairing
	err = tx.SendHeartbeat()
	if err != proto.ErrNotPaired {
		t.Errorf("SendHeartbeat() error = %v, want %v", err, proto.ErrNotPaired)
	}

	// Pairing should be allowed without being paired first
	err = tx.SendFrame(proto.FrameTypePairing, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	if err != nil {
		t.Errorf("SendFrame(Pairing) error = %v, want nil", err)
	}
}

func TestTransmitter_Pairing(t *testing.T) {
	// Create two mock drivers for bidirectional comms
	driverTx := NewMockDriver()
	driverRx := NewMockDriver()

	// Create transmitter and receiver
	txID := proto.DeviceID(0xCAFE)
	rxID := proto.DeviceID(0xBEEF)

	tx := NewTransmitterWithDriver(txID, driverTx)
	rx := NewReceiverWithDriver(rxID, driverRx)

	// Wire the drivers together
	ConnectDrivers(driverTx, driverRx)

	// Start the receiver listening
	rx.Listen()

	// Send pairing request
	err := tx.StartPairing(rxID)
	if err != nil {
		t.Fatalf("StartPairing() error = %v", err)
	}

	// Verify transmitter is now paired
	if !tx.device.IsPaired {
		t.Error("Transmitter not marked as paired")
	}

	// Try to send data now that we're paired
	err = tx.SendData([]byte{1, 2, 3})
	if err != nil {
		t.Errorf("SendData() after pairing error = %v", err)
	}
}

func TestTransmitter_SequenceNumberIncrement(t *testing.T) {
	// Create mock driver
	driver := NewMockDriver()

	// Create paired transmitter
	tx := NewTransmitterWithDriver(0xCAFE, driver)
	tx.device.IsPaired = true

	// Send multiple packets and verify sequence increments
	var prevSeq uint32
	var currSeq uint32

	for i := 0; i < 5; i++ {
		driver.ClearTxLog()

		err := tx.SendFrame(proto.FrameTypeData, []byte{byte(i)})
		if err != nil {
			t.Fatalf("SendFrame() error = %v", err)
		}

		txLog := driver.GetTxLog()
		if len(txLog) == 0 {
			t.Fatal("No packet transmitted")
		}

		sent := proto.DecodeFrame(txLog[0])
		if sent == nil {
			t.Fatal("Transmitted invalid frame")
		}

		if i > 0 {
			prevSeq = currSeq
			currSeq = sent.Seq

			if currSeq != prevSeq+1 {
				t.Errorf("Sequence not incremented correctly: prev=%v, curr=%v", prevSeq, currSeq)
			}
		} else {
			currSeq = sent.Seq
		}
	}
}
