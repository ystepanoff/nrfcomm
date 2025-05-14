package nrfcomm

import (
	"math/rand"
	"time"
	"unsafe"

	"tinygo.org/x/device/nrf"
)

// Transmitter represents a radio transmitter device
type Transmitter struct {
	device     *Device
	buffer     [MaxPayloadSize]byte
	receiver   DeviceID
	pairingKey uint32
}

func NewRadioTransmitter(id DeviceID) *Transmitter {
	rand.Seed(time.Now().UnixNano())

	t := &Transmitter{
		device: NewTransmitter(id),
	}

	t.pairingKey = rand.Uint32()
	t.device.PairingKey = t.pairingKey

	return t
}

func (t *Transmitter) Initialize() {
	StartHFCLK()
	ConfigureRadio(t.device.Address, t.device.Prefix, t.device.Channel)
}

func (t *Transmitter) SetChannel(channel uint8) error {
	if channel > 125 {
		return ErrInvalidChannel
	}
	t.device.Channel = channel
	return nil
}

func (t *Transmitter) SendPacket(packetType byte, payload []byte) error {
	if !t.device.IsPaired && packetType != packetTypePairing {
		return ErrNotPaired
	}

	if len(payload) > (MaxPayloadSize - packetHeaderSize) {
		return ErrInvalidPayload
	}

	packet := &Packet{
		SenderID: t.device.ID,
		Type:     packetType,
		Reserved: [2]byte{0, 0},
		Payload:  payload,
	}

	data := EncodePacket(packet)
	copy(t.buffer[:], data)

	// Set up the packet pointer
	nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&t.buffer[0]))))

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

func (t *Transmitter) StartPairing(receiverID DeviceID) error {
	payload := make([]byte, 8)
	for i := 0; i < 4; i++ {
		payload[i] = byte(t.pairingKey >> (i * 8))
	}
	for i := 0; i < 4; i++ {
		payload[4+i] = byte(receiverID >> (i * 8))
	}

	t.receiver = receiverID

	err := t.SendPacket(packetTypePairing, payload)
	if err != nil {
		return err
	}

	startTime := time.Now().UnixMilli()
	for {
		if time.Now().UnixMilli()-startTime > pairingTimeout {
			return ErrTimeout
		}

		packet := t.ReceivePacket()
		if packet == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if packet.Type == packetTypeAck {
			if len(packet.Payload) >= 4 {
				senderID := DeviceID(0)
				for i := 0; i < 4; i++ {
					senderID |= DeviceID(packet.Payload[i]) << (i * 8)
				}

				if senderID == receiverID {
					t.device.IsPaired = true
					return nil
				}
			}
		}

		// Wrong packet type or invalid ACK, try again
		time.Sleep(100 * time.Millisecond)
	}
}

func (t *Transmitter) ReceivePacket() *Packet {
	nrf.RADIO.PACKETPTR.Set(uint32(uintptr(unsafe.Pointer(&t.buffer[0]))))

	// Clear previous event flags
	nrf.RADIO.EVENTS_READY.Set(0)
	nrf.RADIO.EVENTS_END.Set(0)

	// Enable RX → READY
	nrf.RADIO.TASKS_RXEN.Set(1)
	for nrf.RADIO.EVENTS_READY.Get() == 0 {
	}

	// Start receiving → END with a short timeout
	startTime := time.Now().UnixMilli()
	nrf.RADIO.TASKS_START.Set(1)

	// Wait for packet or timeout (100ms)
	for nrf.RADIO.EVENTS_END.Get() == 0 {
		if time.Now().UnixMilli()-startTime > 100 {
			nrf.RADIO.TASKS_DISABLE.Set(1)
			for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
			}
			return nil
		}
	}

	nrf.RADIO.TASKS_DISABLE.Set(1)
	for nrf.RADIO.STATE.Get() != nrf.RADIO_STATE_STATE_Disabled {
	}

	return DecodePacket(t.buffer[:])
}

func (t *Transmitter) SendHeartbeat() error {
	if !t.device.IsPaired {
		return ErrNotPaired
	}

	return t.SendPacket(packetTypeHeartbeat, nil)
}

func (t *Transmitter) SendData(data []byte) error {
	if !t.device.IsPaired {
		return ErrNotPaired
	}

	return t.SendPacket(packetTypeData, data)
}

func (t *Transmitter) StartHeartbeatTask() {
	go func() {
		for {
			t.SendHeartbeat()
			time.Sleep(heartbeatInterval * time.Millisecond)
		}
	}()
}
