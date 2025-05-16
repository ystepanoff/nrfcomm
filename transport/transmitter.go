package transport

import (
	"log"
	"time"

	proto "github.com/ystepanoff/nrfcomm/protocol"
)

// Transmitter encapsulates high-level logic for a radio transmitter.
type Transmitter struct {
	device     *proto.Device
	driver     RadioDriver
	seq        uint32
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

func (t *Transmitter) SendFrame(FrameType byte, payload []byte) error {
	if !t.device.IsPaired && FrameType != proto.FrameTypePairing {
		return proto.ErrNotPaired
	}
	if len(payload) > proto.MaxPayloadSize {
		return proto.ErrInvalidPayload
	}

	seq := t.seq
	t.seq++

	frame := &proto.Frame{
		SenderID: t.device.ID,
		Type:     FrameType,
		Seq:      seq,
		Payload:  payload,
	}

	return t.driver.Tx(proto.EncodeFrame(frame))
}

func (t *Transmitter) ReceiveFrame(timeout time.Duration) *proto.Frame {
	data, err := t.driver.Rx(timeout)
	if err != nil {
		return nil
	}
	return proto.DecodeFrame(data)
}

func (t *Transmitter) StartPairing(receiverID proto.DeviceID) error {
	// payload: pairingKey(4) | receiverID(4)
	buf := make([]byte, 8)
	for i := 0; i < 4; i++ {
		buf[i] = byte(t.pairingKey >> (i * 8))
		buf[4+i] = byte(receiverID >> (i * 8))
	}
	t.receiver = receiverID

	// remember sequence number that will be used in this pairing Frame
	seq := t.seq

	if err := t.SendFrame(proto.FrameTypePairing, buf); err != nil {
		return err
	}

	deadline := time.Now().Add(proto.PairingTimeout * time.Millisecond)
	for time.Now().Before(deadline) {
		frame := t.ReceiveFrame(100 * time.Millisecond)
		if frame == nil {
			continue
		}
		if frame.Type == proto.FrameTypeAck && frame.Seq == seq && len(frame.Payload) >= 4 {
			sid := proto.DeviceID(uint32(frame.Payload[0]) | uint32(frame.Payload[1])<<8 | uint32(frame.Payload[2])<<16 | uint32(frame.Payload[3])<<24)
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
	err := t.SendFrame(proto.FrameTypeHeartbeat, nil)
	if err == nil {
		log.Printf("[Transmitter] Heartbeat sent (seq=%d)\r\n", t.seq-1)
	}
	return err
}

func (t *Transmitter) SendData(data []byte) error {
	if !t.device.IsPaired {
		return proto.ErrNotPaired
	}
	return t.SendFrame(proto.FrameTypeData, data)
}

func (t *Transmitter) StartHeartbeatTask() {
	go func() {
		ticker := time.NewTicker(proto.HeartbeatInterval * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			_ = t.SendHeartbeat()
		}
	}()
}

// SendDataReliable sends data with acknowledgment and automatic retries.
// It will attempt to send the Frame up to maxRetries times, waiting for an ACK
// with the matching sequence number after each attempt.
func (t *Transmitter) SendDataReliable(data []byte, maxRetries int) error {
	if !t.device.IsPaired {
		return proto.ErrNotPaired
	}

	if len(data) > proto.MaxPayloadSize {
		return proto.ErrInvalidPayload
	}

	// Make a copy of the data to prevent modification during transmission
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	seq := t.seq
	t.seq++

	Frame := &proto.Frame{
		SenderID: t.device.ID,
		Type:     proto.FrameTypeData,
		Seq:      seq,
		Payload:  dataCopy,
	}

	encodedFrame := proto.EncodeFrame(Frame)

	if len(encodedFrame) < proto.FrameHeaderSize {
		return proto.ErrInvalidPayload
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := t.driver.Tx(encodedFrame); err != nil {
			return err
		}

		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			frame := t.ReceiveFrame(20 * time.Millisecond)
			if frame == nil || frame.Payload == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if frame.Type == proto.FrameTypeAck {
				if frame.Seq == seq {
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
