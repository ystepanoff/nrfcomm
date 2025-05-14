//go:build tinygo || baremetal

package nrfcomm

import (
	"sync"
	"time"
	"unsafe"

	"device/nrf"
)

// Receiver represents a radio receiver device
type Receiver struct {
	device        *Device
	buffer        [MaxPayloadSize]byte
	pairedDevices map[DeviceID]*Device
	mu            sync.Mutex
	callbacks     map[byte]func(*Packet)
	isListening   bool
}

func NewRadioReceiver(id DeviceID) *Receiver {
	return &Receiver{
		device:        NewReceiver(id),
		pairedDevices: make(map[DeviceID]*Device),
		callbacks:     make(map[byte]func(*Packet)),
	}
}

func (r *Receiver) Initialise() {
	StartHFCLK()
	ConfigureRadio(r.device.Address, r.device.Prefix, r.device.Channel)
}

func (r *Receiver) RegisterCallback(packetType byte, callback func(*Packet)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callbacks[packetType] = callback
}

func (r *Receiver) ProcessPacket(packet *Packet) {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, isPaired := r.pairedDevices[packet.SenderID]

	// Process based on packet type
	switch packet.Type {
	case packetTypePairing:
		if len(packet.Payload) >= 8 {
			pairingKey := uint32(0)
			for i := 0; i < 4; i++ {
				pairingKey |= uint32(packet.Payload[i]) << (i * 8)
			}

			targetID := DeviceID(0)
			for i := 0; i < 4; i++ {
				targetID |= DeviceID(packet.Payload[4+i]) << (i * 8)
			}

			if targetID == r.device.ID {
				if device == nil {
					device = NewTransmitter(packet.SenderID)
				}

				device.PairingKey = pairingKey
				device.IsPaired = true
				device.UpdateLastSeen()

				r.pairedDevices[packet.SenderID] = device

				r.SendAck(packet.SenderID)
			}
		}

	case packetTypeHeartbeat:
		if isPaired {
			device.UpdateLastSeen()
		}

	case packetTypeData:
		if isPaired {
			device.UpdateLastSeen()

			if callback, ok := r.callbacks[packetTypeData]; ok && callback != nil {
				callback(packet)
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
			packet := r.ReceivePacket()
			if packet != nil {
				r.ProcessPacket(packet)
			}
		}
	}()
}

func (r *Receiver) StopListening() {
	r.isListening = false
}

func (r *Receiver) ReceivePacket() *Packet {
	nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&r.buffer[0]))))

	nrf.RADIO.EVENTS_READY.Set(0)
	nrf.RADIO.EVENTS_END.Set(0)

	nrf.RADIO.TASKS_RXEN.Set(1)
	for nrf.RADIO.EVENTS_READY.Get() == 0 {
	}

	nrf.RADIO.TASKS_START.Set(1)
	for nrf.RADIO.EVENTS_END.Get() == 0 {
	}

	nrf.RADIO.TASKS_DISABLE.Set(1)
	for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
	}

	return DecodePacket(r.buffer[:])
}

func (r *Receiver) IsPaired(deviceID DeviceID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, ok := r.pairedDevices[deviceID]
	return ok && device.IsPaired
}

func (r *Receiver) GetPairedDevices() []*Device {
	r.mu.Lock()
	defer r.mu.Unlock()

	devices := make([]*Device, 0, len(r.pairedDevices))
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
		if (now - device.LastSeen) > deviceTimeout {
			device.IsPaired = false
			delete(r.pairedDevices, id)
		}
	}
}

func (r *Receiver) StartCleanupTask() {
	go func() {
		for {
			time.Sleep(deviceTimeout * time.Millisecond / 2)
			r.CleanupDeadDevices()
		}
	}()
}

func (r *Receiver) SetChannel(channel uint8) error {
	if channel > 125 {
		return ErrInvalidChannel
	}
	r.device.Channel = channel
	return nil
}

func (r *Receiver) SendAck(deviceID DeviceID) error {
	payload := make([]byte, 4)
	for i := 0; i < 4; i++ {
		payload[i] = byte(r.device.ID >> (i * 8))
	}

	packet := &Packet{
		SenderID: r.device.ID,
		Type:     packetTypeAck,
		Reserved: [2]byte{0, 0},
		Payload:  payload,
	}

	data := EncodePacket(packet)
	copy(r.buffer[:], data)

	// Set up the packet pointer
	nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&r.buffer[0]))))

	// Clear previous event flags
	nrf.RADIO.EVENTS_READY.Set(0)
	nrf.RADIO.EVENTS_END.Set(0)

	// Enable TX → READY
	nrf.RADIO.TASKS_TXEN.Set(1)
	for nrf.RADIO.EVENTS_READY.Get() == 0 {
	}

	// Start one packet → END
	nrf.RADIO.TASKS_START.Set(1)
	for nrf.RADIO.EVENTS_END.Get() == 0 {
	}

	// Disable the radio
	nrf.RADIO.TASKS_DISABLE.Set(1)
	for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
	}

	return nil
}

func (r *Receiver) StartPairing() error {
	wasListening := r.isListening
	if !r.isListening {
		r.isListening = true
	}

	startTime := time.Now().UnixMilli()

	for {
		if time.Now().UnixMilli()-startTime > pairingTimeout {
			if !wasListening {
				r.isListening = false
			}
			return ErrTimeout
		}

		packet := r.ReceivePacket()
		if packet == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if packet.Type == packetTypePairing {
			r.ProcessPacket(packet)

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

		time.Sleep(10 * time.Millisecond)
	}
}

func (r *Receiver) GetPairedDeviceID() DeviceID {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id := range r.pairedDevices {
		return id
	}
	return 0
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
		return nil, ErrNotPaired
	}

	startTime := time.Now().UnixMilli()

	for {
		if time.Now().UnixMilli()-startTime > 5000 {
			return nil, ErrTimeout
		}

		packet := r.ReceivePacket()
		if packet == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		r.ProcessPacket(packet)

		if packet.Type == packetTypeData {
			r.mu.Lock()
			_, isPaired := r.pairedDevices[packet.SenderID]
			r.mu.Unlock()

			if isPaired {
				return packet.Payload, nil
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (r *Receiver) StartHeartbeatTask() {
	go func() {
		for {
			r.CleanupDeadDevices()
			time.Sleep(heartbeatInterval * time.Millisecond / 2)
		}
	}()
}
