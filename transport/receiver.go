package transport

import (
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
	callbacks     map[byte]func(*proto.Packet)
	isListening   bool
}

func NewReceiverWithDriver(id proto.DeviceID, d RadioDriver) *Receiver {
	return &Receiver{
		device:        proto.NewReceiver(id),
		driver:        d,
		pairedDevices: make(map[proto.DeviceID]*proto.Device),
		callbacks:     make(map[byte]func(*proto.Packet)),
	}
}

func (r *Receiver) Initialise() {
	r.driver.StartHFCLK()
	_ = r.driver.Configure(r.device.Address, r.device.Prefix, r.device.Channel)
}

func (r *Receiver) RegisterCallback(ptype byte, cb func(*proto.Packet)) {
	r.mu.Lock()
	r.callbacks[ptype] = cb
	r.mu.Unlock()
}

func (r *Receiver) ProcessPacket(pkt *proto.Packet) {
	if pkt == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	dev, paired := r.pairedDevices[pkt.SenderID]

	switch pkt.Type {
	case proto.PacketTypePairing:
		if pkt.Payload != nil && len(pkt.Payload) >= 8 {
			key := uint32(pkt.Payload[0]) | uint32(pkt.Payload[1])<<8 | uint32(pkt.Payload[2])<<16 | uint32(pkt.Payload[3])<<24
			targetID := proto.DeviceID(uint32(pkt.Payload[4]) | uint32(pkt.Payload[5])<<8 | uint32(pkt.Payload[6])<<16 | uint32(pkt.Payload[7])<<24)
			if targetID == r.device.ID {
				if dev == nil {
					dev = proto.NewTransmitter(pkt.SenderID)
				}
				dev.PairingKey = key
				dev.IsPaired = true
				dev.UpdateLastSeen()
				r.pairedDevices[pkt.SenderID] = dev
				_ = r.SendAck(pkt.SenderID)
			}
		}
	case proto.PacketTypeHeartbeat:
		if paired {
			dev.UpdateLastSeen()
		}
	case proto.PacketTypeData:
		if paired && pkt.Payload != nil {
			dev.UpdateLastSeen()

			seqBytes := pkt.Reserved

			localDeviceID := r.device.ID
			localDriver := r.driver
			go func(senderID, responderID proto.DeviceID, seqData [2]byte, driver RadioDriver) {
				seqDataCopy := [2]byte{seqData[0], seqData[1]}

				ackPkt := &proto.Packet{
					SenderID: responderID,
					Type:     proto.PacketTypeAck,
					Reserved: [2]byte{0, 0},
					Payload:  []byte{seqDataCopy[0], seqDataCopy[1]}, // Echo sequence number
				}

				data := proto.EncodePacket(ackPkt)
				if len(data) >= proto.PacketHeaderSize {
					_ = driver.Tx(data)
				}
			}(pkt.SenderID, localDeviceID, seqBytes, localDriver)

			if callback, ok := r.callbacks[proto.PacketTypeData]; ok && callback != nil {
				safePacket := &proto.Packet{
					SenderID: pkt.SenderID,
					Type:     pkt.Type,
					Reserved: pkt.Reserved,
				}

				if pkt.Payload != nil {
					safePacket.Payload = make([]byte, len(pkt.Payload))
					copy(safePacket.Payload, pkt.Payload)
				} else {
					safePacket.Payload = make([]byte, 0)
				}

				callback(safePacket)
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
			pkt := r.ReceivePacket(100 * time.Millisecond)
			if pkt != nil {
				r.ProcessPacket(pkt)
			}
		}
	}()
}

func (r *Receiver) StopListening() { r.isListening = false }

func (r *Receiver) ReceivePacket(timeout time.Duration) *proto.Packet {
	data, err := r.driver.Rx(timeout)
	if err != nil {
		return nil
	}
	return proto.DecodePacket(data)
}

func (r *Receiver) SetChannel(ch uint8) error {
	if ch > 125 {
		return proto.ErrInvalidChannel
	}
	r.device.Channel = ch
	return r.driver.SetChannel(ch)
}

func (r *Receiver) SendAck(to proto.DeviceID) error {
	pl := make([]byte, 4)
	for i := 0; i < 4; i++ {
		pl[i] = byte(r.device.ID >> (i * 8))
	}

	ackPacket := &proto.Packet{
		SenderID: r.device.ID,
		Type:     proto.PacketTypeAck,
		Reserved: [2]byte{0, 0},
		Payload:  pl,
	}

	data := proto.EncodePacket(ackPacket)
	if len(data) < proto.PacketHeaderSize {
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
		pkt := r.ReceivePacket(100 * time.Millisecond)
		if pkt != nil && pkt.Type == proto.PacketTypePairing {
			r.ProcessPacket(pkt)
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
			device.IsPaired = false
			delete(r.pairedDevices, id)
		}
	}
}

func (r *Receiver) StartCleanupTask() {
	go func() {
		for {
			time.Sleep(proto.DeviceTimeout * time.Millisecond / 2)
			r.CleanupDeadDevices()
		}
	}()
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

		packet := r.ReceivePacket(100 * time.Millisecond)
		if packet == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if packet.Payload == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		r.ProcessPacket(packet)

		if packet.Type == proto.PacketTypeData {
			r.mu.Lock()
			_, isPaired := r.pairedDevices[packet.SenderID]
			r.mu.Unlock()

			if isPaired {
				dataCopy := make([]byte, len(packet.Payload))
				copy(dataCopy, packet.Payload)
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
