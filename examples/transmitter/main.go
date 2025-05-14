package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
)

func main() {
	// Create a new transmitter with a unique ID
	transmitter := nrfcomm.NewRadioTransmitter(0x12345678)

	// Set custom channel (0-125, default is 7)
	// Each channel is 1MHz wide, starting at 2400MHz
	// So channel 7 = 2407MHz, channel 80 = 2480MHz, etc.
	if err := transmitter.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	// Initialize the radio
	transmitter.Initialize()

	// Try to pair with a receiver (ID: 0x87654321)
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

	// Start sending heartbeats in the background
	transmitter.StartHeartbeatTask()

	// Main loop: send a message every second
	counter := uint32(0)
	for {
		// Create a simple payload with a counter
		payload := []byte{
			byte(counter & 0xFF),
			byte((counter >> 8) & 0xFF),
			byte((counter >> 16) & 0xFF),
			byte((counter >> 24) & 0xFF),
		}

		// Send the data
		err := transmitter.SendData(payload)
		if err != nil {
			println("Failed to send data:", err.Error())
		} else {
			println("Sent data packet:", counter)
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}
