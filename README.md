# nrfcomm

A 2.4GHz radio communication interface for nice!nano boards using TinyGo.

## Features

* Channel selection (0-125)
* Device pairing with ACK support
* Heartbeat monitoring
* Data packet transmission/reception
* Automatic connection management

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
	// Create a new transmitter with a unique ID
	transmitter := nrfcomm.NewRadioTransmitter(0x12345678)

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
		time.Sleep(1 * time.Second)
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
	// Create a new receiver with a unique ID
	receiver := nrfcomm.NewRadioReceiver(0x87654321)

	// Set custom channel (0-125, default is 7)
	if err := receiver.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	// Initiasise the radio
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
```

## Building and Flashing

To compile and flash your code to a nice!nano board:

```bash
tinygo flash -target=nice_nano -size=short path/to/your/code
```

## License

MIT 