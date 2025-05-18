//go:build tinygo || baremetal

package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
)

func main() {
	time.Sleep(3 * time.Second)

	transmitter := nrfcomm.NewTransmitter(0x12345678)

	if err := transmitter.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	transmitter.Initialise()

	println("Attempting to pair with receiver 0x87654321...")
	err := transmitter.StartPairing(0x87654321)
	if err != nil {
		if err == nrfcomm.ErrTimeout {
			println("Pairing timed out: No response from receiver")
		} else {
			println("Pairing failed:", err.Error())
		}
		return
	}

	println("Paired successfully! Receiver acknowledged the pairing.")

	transmitter.StartHeartbeatTask()

	counter := uint32(0)
	for {
		payload := []byte{
			byte(counter & 0xFF),
			byte((counter >> 8) & 0xFF),
			byte((counter >> 16) & 0xFF),
			byte((counter >> 24) & 0xFF),
		}

		err := transmitter.SendData(payload)
		if err != nil {
			println("Failed to send data:", err.Error())
		} else {
			println("Sent data packet:", counter)
		}

		counter++
		time.Sleep(10 * time.Millisecond)
	}
}
