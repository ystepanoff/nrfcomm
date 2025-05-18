//go:build tinygo || baremetal

package main

import (
	"time"

	"github.com/ystepanoff/nrfcomm"
	"github.com/ystepanoff/nrfcomm/protocol"
)

func main() {
	time.Sleep(3 * time.Second)

	receiver := nrfcomm.NewReceiver(0x87654321)

	if err := receiver.SetChannel(80); err != nil {
		println("Failed to set channel:", err.Error())
		return
	}

	println("Waiting for pairing requests...")
	println("When a pairing request is received, will automatically send ACK")

	receiver.Initialise()

	receiver.RegisterCallback(nrfcomm.FrameTypeData, func(frame *protocol.Frame) {
		println("Received data packet:", frame.Payload[0])
	})

	receiver.Listen()
	receiver.StartCleanupTask()

	for {
		time.Sleep(time.Hour)
	}
}
