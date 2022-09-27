package main

import (
	"fmt"
	"time"
)

type TCPHeader struct {
	sourcePort            int16
	destinationPort       int16
	sequenceNumber        int
	acknowledgementNumber int
	data                  string
}

func listen(serverch chan string, clientch chan string, tcpheader TCPHeader) {
	input := <-serverch
	if input == "passive open" {
		fmt.Printf("SERVER %d -- State: Listen \n", tcpheader.destinationPort)
		time.Sleep(time.Second)
	}
}
func synsent(serverch chan string, clientch chan string, tcpheader TCPHeader) {
	input := <-clientch
	if input == "active open" {
		fmt.Printf("CLIENT %d-- State: SYN SENT \n       -- sending syn (%d) \n", tcpheader.sourcePort, tcpheader.sequenceNumber)
		serverch <- "syn"
		time.Sleep(time.Second)
		synrcvd(serverch, clientch, tcpheader)
	} else if input == "syn, ack" {
		fmt.Printf("CLIENT %d -- State: SYN SENT \n       -- received: syn (%d), ack sending: ack (%d) \n", tcpheader.sourcePort, tcpheader.sequenceNumber, tcpheader.acknowledgementNumber)
		serverch <- "ack"
		time.Sleep(time.Second)
		established(serverch, clientch, tcpheader)

	} else if input == "close" {
		fmt.Printf("CLIENT %d-- State: SYN SENT \n       -- received : close \n", tcpheader.sourcePort)
		clientch <- "close"
		time.Sleep(time.Second)
		closed(serverch, clientch, tcpheader)
	}
}

func synrcvd(serverch chan string, clientch chan string, tcpheader TCPHeader) {
	select {
	case input := <-serverch:
		if input == "syn" {
			fmt.Printf("SERVER %d-- State: SYN RCVD \n       -- received : syn (%d) sending: syn, ack (%d,%d) \n", tcpheader.destinationPort, tcpheader.sequenceNumber, tcpheader.sequenceNumber, tcpheader.acknowledgementNumber)
			clientch <- "syn, ack"
			time.Sleep(time.Second)
			synsent(serverch, clientch, tcpheader)
		} else if input == "ack" {
			time.Sleep(time.Second)
			established(serverch, clientch, tcpheader)
		}
	case input := <-clientch:
		if input == "syn" {
			fmt.Printf("CLIENT %d -- (ESTABLISHED CONNECTION) \n              -- received syn: %d and sends length of data+syn as ack: %d \n \n", tcpheader.destinationPort, tcpheader.sequenceNumber+1, len(tcpheader.data)+tcpheader.sequenceNumber+1)
		}
	}
}
func established(serverch chan string, clientch chan string, tcpheader TCPHeader) {
	input := <-serverch
	if input == "ack" {
		fmt.Printf("SERVER %d/CLIENT %d -- ESTABLISHED CONNECTION \n              -- Exchanging data :"+tcpheader.data+" ... \n", tcpheader.destinationPort, tcpheader.sourcePort)
		time.Sleep(2 * time.Second)
		clientch <- "syn"
		synrcvd(serverch, clientch, tcpheader)
	}
}

func initialize(serverch chan string, clientch chan string, tcpheader TCPHeader) {
	serverch <- "passive open"
	clientch <- "active open"
	listen(serverch, clientch, tcpheader)
	synsent(serverch, clientch, tcpheader)
}
func closed(serverch chan string, clientch chan string, tcpheader TCPHeader) {
	select {
	case <-clientch:
		fmt.Printf("CLIENT %d -- State: Closed. \n", tcpheader.sourcePort)
	case <-serverch:
		fmt.Printf("CLIENT %d -- State: Closed. \n", tcpheader.sourcePort)
	}
}

// Tried to incorporate the State Machine model for the TC protocol...
func main() {
	exData := "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum."
	for i := 0; i < len(exData); i = i + 50 {
		serverchannel := make(chan string, 1)
		clientchannel := make(chan string, 1)
		packet := TCPHeader{sourcePort: 123, destinationPort: 456, sequenceNumber: i, acknowledgementNumber: i + 1, data: exData[i : i+50]}
		initialize(serverchannel, clientchannel, packet)
	}

}
