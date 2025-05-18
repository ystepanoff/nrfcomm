# nrfcomm

A 2.4GHz radio communication interface for nice!nano boards using TinyGo.

## Features

* Channel selection (0-125)
* Device pairing with ACK support
* Heartbeat monitoring
* Data Frame transmission/reception
* Automatic connection management
* Multi-device pairing support

## Important: TinyGo Only

This package uses hardware-specific features only available through TinyGo. It will not compile with standard Go.

### Requirements

* [TinyGo](https://tinygo.org/) compiler
* A nice!nano board or other nRF52-based microcontroller

### Installation

In your TinyGo project:

```bash
go get github.com/ystepanoff/nrfcomm
```

## Usage

### Transmitter Example

```go
//go:build tinygo || baremetal

package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
)

func main() {
	time.Sleep(3 * time.Second)
	// Create a new transmitter with a unique ID
	transmitter := nrfcomm.NewTransmitter(0x12345678)

	// Set custom channel (0-125, default is 7)
	if err := transmitter.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	// Initialise the radio
	transmitter.Initialise()

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
		time.Sleep(10 * time.Millisecond)
	}
}
```

### Receiver Example

```go
//go:build tinygo || baremetal

package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
)

func main() {
	time.Sleep(3 * time.Second)
	// Create a new receiver with a unique ID
	receiver := nrfcomm.NewReceiver(0x87654321)

	// Set custom channel (0-125, default is 7)
	if err := receiver.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	// Initiaise the radio
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

### Continuous Pairing Mode (Multi-Device) Example

The receiver can be configured to remain in a listening state indefinitely, allowing it to pair with multiple transmitters that join over time.

```go
//go:build tinygo || baremetal

package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
)

func main() {
	time.Sleep(3 * time.Second)
	
	// Create a new receiver with a unique ID
	receiver := nrfcomm.NewReceiver(0x87654321)
	
	// Initialise the radio
	receiver.Initialise()
	
	// Register a callback to receive data from any paired device
	receiver.RegisterCallback(nrfcomm.FrameTypeData, func(frame *nrfcomm.Frame) {
		if len(frame.Payload) > 0 {
			// Convert payload bytes to a counter value
			counter := uint32(0)
			if len(frame.Payload) >= 4 {
				counter = binary.LittleEndian.Uint32(frame.Payload)
			} else {
				counter = uint32(frame.Payload[0])
			}
			
			// Print the received data along with the sender's ID
			println("Received data:", counter, "from device:", frame.SenderID)
		}
	})
	
	// Start continuous listening mode - will accept pairing from any matching device ID
	println("Starting continuous listening mode...")
	receiver.Listen()
	
	// Start the heartbeat and cleanup tasks
	receiver.StartCleanupTask()
	
	// Keep the program running
	for {
		// Periodically report connected devices
		devices := receiver.GetPairedDeviceIDs()
		println("Currently paired devices:", len(devices))
		for i, deviceID := range devices {
			println("Device", i, ":", deviceID)
		}
		
		time.Sleep(1 * time.Minute)
	}
}
```

In this mode, the receiver:

1. Starts continuous listening with `Listen()` instead of `StartPairing()`
2. Processes incoming pairing requests automatically from any device
3. Uses the `RegisterCallback` approach to receive data from all paired devices
4. Keeps track of multiple paired devices with `GetPairedDeviceIDs()`
5. Runs a periodic cleanup task that removes devices that haven't sent heartbeats recently

When a transmitter attempts to pair with the receiver's ID, the pairing will succeed automatically and data transmission can begin immediately.

## Building and Flashing

To compile and flash your code to a nice!nano board:

```bash
tinygo flash -target=nicenano -size=short path/to/your/code
```

## License

MIT
