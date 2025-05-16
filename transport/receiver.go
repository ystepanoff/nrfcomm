package transport

import (
	"log"
	"sync"
	"time"

	proto "github.com/ystepanoff/nrfcomm/protocol"
)

// Receiver encapsulates high-level logic for a radio receiver.
type Receiver struct {
	device        *proto.Device
	driver        RadioDriver
	pairedDevices map[proto.DeviceID]*proto.Device
	mu            sync.Mutex
	callbacks     map[byte]func(*proto.Frame)
	isListening   bool
}

func NewReceiverWithDriver(id proto.DeviceID, d RadioDriver) *Receiver {
	return &Receiver{
		device:        proto.NewReceiver(id),
		driver:        d,
		pairedDevices: make(map[proto.DeviceID]*proto.Device),
		callbacks:     make(map[byte]func(*proto.Frame)),
	}
}

func (r *Receiver) Initialise() {
	r.driver.StartHFCLK()
	_ = r.driver.Configure(r.device.Address, r.device.Prefix, r.device.Channel)
}

func (r *Receiver) RegisterCallback(ptype byte, cb func(*proto.Frame)) {
	r.mu.Lock()
	r.callbacks[ptype] = cb
	r.mu.Unlock()
}

func (r *Receiver) ProcessFrame(frame *proto.Frame) {
	if frame == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	dev, paired := r.pairedDevices[frame.SenderID]

	switch frame.Type {
	case proto.FrameTypePairing:
		log.Printf("[Receiver] Pairing Frame received\r\n")
		log.Printf("[Receiver] Payload: %v\r\n", frame.Payload)
		if len(frame.Payload) >= 8 {
			key := uint32(frame.Payload[0]) | uint32(frame.Payload[1])<<8 | uint32(frame.Payload[2])<<16 | uint32(frame.Payload[3])<<24
			targetID := proto.DeviceID(uint32(frame.Payload[4]) | uint32(frame.Payload[5])<<8 | uint32(frame.Payload[6])<<16 | uint32(frame.Payload[7])<<24)
			if targetID == r.device.ID {
				if dev == nil {
					dev = proto.NewTransmitter(frame.SenderID)
				}
				dev.PairingKey = key
				dev.IsPaired = true
				dev.UpdateLastSeen()
				r.pairedDevices[frame.SenderID] = dev
				_ = r.SendAck(frame.SenderID, frame.Seq)
			}
		}
	case proto.FrameTypeHeartbeat:
		if paired {
			dev.UpdateLastSeen()
			log.Printf("[Receiver] Heartbeat received from %d (seq=%d)\r\n", frame.SenderID, frame.Seq)
		}
	case proto.FrameTypeData:
		if paired && frame.Payload != nil {
			dev.UpdateLastSeen()

			// Send ACK immediately (no new goroutine to minimise allocations)
			ackframe := &proto.Frame{
				SenderID: r.device.ID,
				Type:     proto.FrameTypeAck,
				Seq:      frame.Seq,
			}
			_ = r.driver.Tx(proto.EncodeFrame(ackframe))

			// Log ACK sent (use sequence number bytes for clarity)
			log.Printf("[Receiver] ACK sent for seq=%d\r\n", frame.Seq)

			// Invoke callback directly using the same Frame to avoid extra allocations
			if callback, ok := r.callbacks[proto.FrameTypeData]; ok && callback != nil {
				callback(frame)
			}
		}
	}
}

func (r *Receiver) Listen() {
	if r.isListening {
		return
	}
	r.isListening = true
	go func() {
		for r.isListening {
			frame := r.ReceiveFrame(100 * time.Millisecond)
			if frame != nil {
				r.ProcessFrame(frame)
			}
		}
	}()
}

func (r *Receiver) StopListening() { r.isListening = false }

func (r *Receiver) ReceiveFrame(timeout time.Duration) *proto.Frame {
	data, err := r.driver.Rx(timeout)
	if err != nil {
		return nil
	}
	return proto.DecodeFrame(data)
}

func (r *Receiver) SetChannel(ch uint8) error {
	if ch > 125 {
		return proto.ErrInvalidChannel
	}
	r.device.Channel = ch
	return r.driver.SetChannel(ch)
}

func (r *Receiver) SendAck(to proto.DeviceID, seq uint32) error {
	pl := make([]byte, 4)
	for i := 0; i < 4; i++ {
		pl[i] = byte(r.device.ID >> (i * 8))
	}

	ackFrame := &proto.Frame{
		SenderID: r.device.ID,
		Type:     proto.FrameTypeAck,
		Seq:      seq,
		Payload:  pl,
	}

	data := proto.EncodeFrame(ackFrame)
	if len(data) < proto.FrameHeaderSize {
		return proto.ErrInvalidPayload
	}

	return r.driver.Tx(data)
}

func (r *Receiver) StartPairing() error {
	wasListening := r.isListening
	if !r.isListening {
		r.isListening = true
	}
	deadline := time.Now().Add(proto.PairingTimeout * time.Millisecond)
	for time.Now().Before(deadline) {
		frame := r.ReceiveFrame(100 * time.Millisecond)
		if frame != nil && frame.Type == proto.FrameTypePairing {
			r.ProcessFrame(frame)
			r.mu.Lock()
			paired := len(r.pairedDevices) > 0
			r.mu.Unlock()
			if paired {
				if !wasListening {
					r.isListening = false
				}
				return nil
			}
		}
	}
	if !wasListening {
		r.isListening = false
	}
	return proto.ErrTimeout
}

func (r *Receiver) IsPaired(deviceID proto.DeviceID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, ok := r.pairedDevices[deviceID]
	return ok && device.IsPaired
}

func (r *Receiver) GetPairedDevices() []*proto.Device {
	r.mu.Lock()
	defer r.mu.Unlock()

	devices := make([]*proto.Device, 0, len(r.pairedDevices))
	for _, device := range r.pairedDevices {
		if device.IsPaired {
			devices = append(devices, device)
		}
	}

	return devices
}

func (r *Receiver) CleanupDeadDevices() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()

	for id, device := range r.pairedDevices {
		if (now - device.LastSeen) > proto.DeviceTimeout {
			log.Printf("[Receiver] Device %d timed out\r\n", id)
			device.IsPaired = false
			delete(r.pairedDevices, id)
		}
	}
}

func (r *Receiver) GetPairedDeviceID() proto.DeviceID {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id := range r.pairedDevices {
		return id
	}
	return 0
}

func (r *Receiver) GetPairedDeviceIDs() []proto.DeviceID {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := make([]proto.DeviceID, 0, len(r.pairedDevices))
	for id := range r.pairedDevices {
		ids = append(ids, id)
	}
	return ids
}

func (r *Receiver) IsPairedDeviceConnected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, device := range r.pairedDevices {
		if device.IsAlive() {
			return true
		}
	}
	return false
}

func (r *Receiver) ReceiveData() ([]byte, error) {
	if len(r.pairedDevices) == 0 {
		return nil, proto.ErrNotPaired
	}

	startTime := time.Now().UnixMilli()

	for {
		if time.Now().UnixMilli()-startTime > 5000 {
			return nil, proto.ErrTimeout
		}

		Frame := r.ReceiveFrame(100 * time.Millisecond)
		if Frame == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if Frame.Payload == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		r.ProcessFrame(Frame)

		if Frame.Type == proto.FrameTypeData {
			r.mu.Lock()
			_, isPaired := r.pairedDevices[Frame.SenderID]
			r.mu.Unlock()

			if isPaired {
				dataCopy := make([]byte, len(Frame.Payload))
				copy(dataCopy, Frame.Payload)
				return dataCopy, nil
			}
		}
	}
}

func (r *Receiver) StartHeartbeatTask() {
	go func() {
		for {
			r.CleanupDeadDevices()
			time.Sleep(proto.HeartbeatInterval * time.Millisecond / 2)
		}
	}()
}
