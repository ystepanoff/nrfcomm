package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
)

func main() {
	// Create a new receiver with a unique ID
	receiver := nrfcomm.NewRadioReceiver(0x87654321)

	// Set custom channel (0-125, default is 7)
	// Each channel is 1MHz wide, starting at 2400MHz
	// So channel 7 = 2407MHz, channel 80 = 2480MHz, etc.
	if err := receiver.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	// Initialize the radio
	receiver.Initialise()

	// Start listening for pairing requests
	println("Waiting for pairing requests...")
	println("When a pairing request is received, will automatically send ACK")
	err := receiver.StartPairing()
	if err != nil {
		println("Pairing failed:", err.Error())
		return
	}

	println("Paired successfully with device ID:", receiver.GetPairedDeviceID())

	// Start the heartbeat monitoring task
	receiver.StartHeartbeatTask()

	// Main loop: receive data
	for {
		// Wait for data
		data, err := receiver.ReceiveData()
		if err != nil {
			println("Failed to receive data:", err.Error())

			// Check if device is still connected
			if !receiver.IsPairedDeviceConnected() {
				println("Device disconnected!")
				// You could attempt to re-pair here or take other action
			}

			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Process received data (in this example, interpret as a counter)
		counter := uint32(data[0]) | (uint32(data[1]) << 8) | (uint32(data[2]) << 16) | (uint32(data[3]) << 24)
		println("Received data packet:", counter)
	}
}
