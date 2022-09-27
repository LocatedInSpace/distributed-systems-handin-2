package main

import (
	"bufio"
	"fmt"
	"handin2/packet"
	"net"
	"os"
	"strings"
	"time"
)

var id = byte('c')

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

	// ------------------- EXAMPLE 1, TALKING TO ECHO SERVER -------------
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("-> ")
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\n", "", -1)
		fmt.Printf("--------------------------\n| Sending data to 's'\n_______\n| %s\n--------------------------\n", text)
		err = packet.Send(c, id, 's', []byte(text), 1, 10)
		//err = packet.Send(c, id, 's', []byte(text), uint16(len(text)), 10)
		if err != nil {
			fmt.Println(err)
			continue
		}

		data, _, err := packet.Recv(c, id)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("| Received from 's'\n_______\n| %s\n--------------------------\n", string(data))
	}

	// ------------------- EXAMPLE 2, PINGING TO DETERMINE LATENCY -------------
	// for {
	// 	// notice how we are timing the Send wrapper, which has receive inside of it
	// 	// we are really timing the time it took to send the message, and that we received
	// 	// the confirmation from server - so it's roundway trip
	// 	start := -time.Now().UnixMilli()
	// 	err = packet.Send(c, id, 's', []byte{}, 0, 0)
	// 	start += time.Now().UnixMilli()
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		return
	// 	}
	// 	fmt.Printf("Pinging 's' took %v ms\n", start)
	// }

	c.Close()
}
