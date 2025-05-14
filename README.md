# nrfcomm

nrfcomm is a TinyGo package for 2.4GHz radio communication between boards that use the nRF52840 processor.

## Features

* Automatic pairing between transmitter and receiver
* Support for multiple transmitters paired to a single receiver
* Raw packet transmission (up to  56 bytes payload)
* Heartbeat mechanism for detecting disconnected/dead devices
* Event-based callback system for handling data packets

## Usage

### Transmitter Example

```go
package main

import (
	"time"
	
	"github.com/ystepanoff/nrfcomm"
)

func main() {
	// Create a new transmitter with a unique ID
	transmitter := nrfcomm.NewRadioTransmitter(0x12345678)
	
	// Initialize the radio
	transmitter.Initialise()
	
	// Pair with a specific receiver
	err := transmitter.StartPairing(0x87654321)
	if err != nil {
		println("Pairing failed:", err.Error())
		return
	}
	
	// Start sending heartbeats automatically
	transmitter.StartHeartbeatTask()
	
	// Send data periodically
	counter := uint32(0)
	for {
		// Create payload
		payload := []byte{
			byte(counter & 0xFF),
			byte((counter >> 8) & 0xFF),
			byte((counter >> 16) & 0xFF),
			byte((counter >> 24) & 0xFF),
		}
		
		transmitter.SendData(payload)
		counter++
		time.Sleep(1 * time.Second)
	}
}
```

### Receiver Example

```go
package main

import (
	"fmt"
	"time"
	
	"github.com/yourusername/nrfcomm"
)

func main() {
	// Create a new receiver with a unique ID
	receiver := nrfcomm.NewRadioReceiver(0x87654321)
	
	// Initialize the radio
	receiver.Initialize()
	
	// Register a callback for data packets
	receiver.RegisterCallback(0x02, func(packet *nrfcomm.Packet) {
		fmt.Printf("Received data from device %08X\n", packet.SenderID)
	})
	
	// Start the dead device cleanup task
	receiver.StartCleanupTask()
	
	// Start listening for packets
	receiver.Listen()
	
	// Main loop
	for {
		time.Sleep(5 * time.Second)
	}
}
```

## License

MIT License

## Acknowledgments

Based on the Nordic Semiconductor nRF52840 radio examples. 