package transport

import (
	"time"

	proto "github.com/ystepanoff/nrfcomm/protocol"
)

// Transmitter encapsulates high-level logic for a radio transmitter.
type Transmitter struct {
	device     *proto.Device
	driver     RadioDriver
	seq        uint16
	receiver   proto.DeviceID
	pairingKey uint32
}

func NewTransmitterWithDriver(id proto.DeviceID, d RadioDriver) *Transmitter {
	pk := proto.GeneratePairingKey()
	t := &Transmitter{
		device:     proto.NewTransmitter(id),
		driver:     d,
		pairingKey: pk,
	}
	t.device.PairingKey = pk
	return t
}

func (t *Transmitter) Initialise() {
	t.driver.StartHFCLK()
	_ = t.driver.Configure(t.device.Address, t.device.Prefix, t.device.Channel)
}

func (t *Transmitter) SetChannel(ch uint8) error {
	if ch > 125 {
		return proto.ErrInvalidChannel
	}
	t.device.Channel = ch
	return t.driver.SetChannel(ch)
}

func (t *Transmitter) SendPacket(packetType byte, payload []byte) error {
	if !t.device.IsPaired && packetType != proto.PacketTypePairing {
		return proto.ErrNotPaired
	}
	if len(payload) > (proto.MaxPayloadSize - proto.PacketHeaderSize) {
		return proto.ErrInvalidPayload
	}

	seq := t.seq
	t.seq++

	pkt := &proto.Packet{
		SenderID: t.device.ID,
		Type:     packetType,
		Reserved: [2]byte{byte(seq), byte(seq >> 8)},
		Payload:  payload,
	}

	return t.driver.Tx(proto.EncodePacket(pkt))
}

func (t *Transmitter) ReceivePacket(timeout time.Duration) *proto.Packet {
	data, err := t.driver.Rx(timeout)
	if err != nil {
		return nil
	}
	return proto.DecodePacket(data)
}

func (t *Transmitter) StartPairing(receiverID proto.DeviceID) error {
	// payload: pairingKey(4) | receiverID(4)
	buf := make([]byte, 8)
	for i := 0; i < 4; i++ {
		buf[i] = byte(t.pairingKey >> (i * 8))
		buf[4+i] = byte(receiverID >> (i * 8))
	}
	t.receiver = receiverID

	if err := t.SendPacket(proto.PacketTypePairing, buf); err != nil {
		return err
	}

	deadline := time.Now().Add(proto.PairingTimeout * time.Millisecond)
	for time.Now().Before(deadline) {
		pkt := t.ReceivePacket(100 * time.Millisecond)
		if pkt == nil {
			continue
		}
		if pkt.Type == proto.PacketTypeAck && len(pkt.Payload) >= 4 {
			sid := proto.DeviceID(uint32(pkt.Payload[0]) | uint32(pkt.Payload[1])<<8 | uint32(pkt.Payload[2])<<16 | uint32(pkt.Payload[3])<<24)
			if sid == receiverID {
				t.device.IsPaired = true
				return nil
			}
		}
	}
	return proto.ErrTimeout
}

func (t *Transmitter) SendHeartbeat() error {
	if !t.device.IsPaired {
		return proto.ErrNotPaired
	}
	return t.SendPacket(proto.PacketTypeHeartbeat, nil)
}

func (t *Transmitter) SendData(data []byte) error {
	if !t.device.IsPaired {
		return proto.ErrNotPaired
	}
	return t.SendPacket(proto.PacketTypeData, data)
}

func (t *Transmitter) StartHeartbeatTask() {
	go func() {
		for {
			_ = t.SendHeartbeat()
			time.Sleep(proto.HeartbeatInterval * time.Millisecond)
		}
	}()
}

// SendDataReliable sends data with acknowledgment and automatic retries.
// It will attempt to send the packet up to maxRetries times, waiting for an ACK
// with the matching sequence number after each attempt.
func (t *Transmitter) SendDataReliable(data []byte, maxRetries int) error {
	if !t.device.IsPaired {
		return proto.ErrNotPaired
	}

	if len(data) > (proto.MaxPayloadSize - proto.PacketHeaderSize) {
		return proto.ErrInvalidPayload
	}

	// Make a copy of the data to prevent modification during transmission
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	seq := t.seq
	t.seq++

	packet := &proto.Packet{
		SenderID: t.device.ID,
		Type:     proto.PacketTypeData,
		Reserved: [2]byte{byte(seq), byte(seq >> 8)}, // Sequence number
		Payload:  dataCopy,
	}

	encodedPacket := proto.EncodePacket(packet)

	if len(encodedPacket) < proto.PacketHeaderSize {
		return proto.ErrInvalidPayload
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := t.driver.Tx(encodedPacket); err != nil {
			return err
		}

		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			pkt := t.ReceivePacket(20 * time.Millisecond)
			if pkt == nil || pkt.Payload == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if pkt.Type == proto.PacketTypeAck {
				if len(pkt.Payload) >= 2 &&
					pkt.Payload[0] == byte(seq) &&
					pkt.Payload[1] == byte(seq>>8) {
					return nil // Success!
				}
			}
			time.Sleep(10 * time.Millisecond)
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(20+(attempt*10)) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return proto.ErrTimeout
}
