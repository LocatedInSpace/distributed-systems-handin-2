package main

import (
	"fmt"
	"handin2/packet"
	"net"
	"os"
	"time"
)

var id = byte('s')

// to simplify working with forwarder, pseudo_server only works with one pseudo_client
// this can be remedied by use of equivalents to handleReceive/handleSend in packet.go
// but this increases complexity by a lot
func main() {
	arguments := os.Args
	CONNECT := ""
	if len(arguments) == 1 {
		fmt.Println("Using default localhost:4004")
		CONNECT = "localhost:4004"
	} else {
		CONNECT = arguments[1]
	}

	c, err := net.Dial("tcp", CONNECT)
	if err != nil {
		fmt.Println(err)
		return
	}

	// the first message to forwarder (which acts as 'the internet') will be the
	// name/id of our pseudo_client/server, this is due to the fact that the forwarder
	// also acts as a pseudo ARP - this name we supply it will be how it knows which
	// connection to send data to when another connection asks to send data there
	c.Write([]byte{id})
	// ^ in my implementation, this id can only be one character

	// for some reason, at least on my machine, without a litle sleep, it would arbitrarily
	// just not receive anything in forwarder
	time.Sleep(50 * time.Millisecond)

	// ------------ EXAMPLE 1, ECHO SERVER ---------------
	for {
		data, dest, err := packet.Recv(c, id, 0)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Received from <%c>: %s\nSending it back...\n", dest, string(data))

		err = packet.Send(c, id, dest, data, uint16(len(data)), 3)
		if err != nil {
			fmt.Println(err)
		}
	}

	// ------------ EXAMPLE 2, RECV/LISTEN/PING SERVER ---------------
	// for {
	// 	_, dest, err := packet.Recv(c, id, 0)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		return
	// 	}
	// 	fmt.Printf("Received ping from <%c>\n", dest)
	// }
}
