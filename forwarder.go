// adapted from concurrent tcp server
// https://www.linode.com/docs/guides/developing-udp-and-tcp-clients-and-servers-in-go/
package main

import (
	"fmt"
	"handin2/packet"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

var verbose bool = true

// network has bad jitter
var jitter bool = false

// percentage for packet to be dropped, ignored
var drop_packet int = 0

// percentage for packet to have a byte zeroed
var zero_bit int = 0

// make sure we dont update packets in different goroutines
var m sync.Mutex

// id -> packets that need to be sent to it
var packets = make(map[byte][][]byte)

// should the handleSend goroutine exit
var isClosed = make(map[byte]bool)

func handleSend(c net.Conn, id byte) {
	// sleep is used to simulate latency, ideally
	// packets received (to be sent) would use a channel
	// to not waste time waiting for data, etc.
	var p []byte
	for {
		time.Sleep(10 * time.Millisecond)
		if isClosed[id] {
			isClosed[id] = false
			return
		}
		m.Lock()
		if len(packets[id]) > 0 {
			// shuffling array to simulate really bad jitter
			// model implementation can (sometimes) survive this attack
			if jitter {
				dest := make([][]byte, len(packets[id]))
				perm := rand.Perm(len(packets[id]))
				for i, v := range perm {
					dest[v] = packets[id][i]
				}
				p, packets[id] = dest[0], dest[1:]
			} else {
				p, packets[id] = packets[id][0], packets[id][1:]
			}
			n := rand.Intn(100)
			if drop_packet > n {
				fmt.Printf("handleRecv<%c> dropped - <%s>\n", id, packet.FmtBits(p))
			} else {
				n = rand.Intn(100)
				if zero_bit > n {
					fmt.Printf("handleRecv<%c> flipped some bits\n", id)
					p[len(p)-3] &= 0x00
				}
				c.Write(p)
			}
		}
		m.Unlock()
	}
}

func handleReceive(c net.Conn) {
	// 65543 is max size of our 'packet'
	data := make([]byte, 65543)
	//id_info, err := bufio.NewReader(c).ReadBytes('\n')
	_, err := c.Read(data)
	id := data[0]
	fmt.Printf("+ Connection from <%c>\n", id)
	if err != nil {
		fmt.Println(err)
		return
	}
	// it is assumed that there will be no duplicate registrations with forwarder
	// as in, no malicious actor that takes advantage of it being a model
	if verbose {
		fmt.Printf("Started handleSend for <%c>\n", id)
	}
	go handleSend(c, id)

	for {
		// the reason we make a new variable, rather than reuse data
		// is because we want to store slices into it on the packets map
		// if we reuse data, then they will all point to the same value (identical entries)
		buffer := make([]byte, 65543)
		n, err := c.Read(buffer)
		if err != nil {
			fmt.Println(err)
			goto errored
		}

		// note, we dont use valid, since its not the forwarders responsibility
		corrupt, _, dest, _, _, flag, _, _ := packet.Decode(buffer[:n])
		if verbose {
			switch flag {
			case packet.START:
				fmt.Printf("handleRecv<%c> - START PACKET to <%c>\n", id, dest)
			case packet.ACCEPT:
				fmt.Printf("handleRecv<%c> - ACCEPT PACKET to <%c>\n", id, dest)
			case packet.DONE:
				fmt.Printf("handleRecv<%c> - DONE PACKET to <%c>\n", id, dest)
			case packet.DONE | packet.ACCEPT:
				fmt.Printf("handleRecv<%c> - ACCEPT & DONE PACKET to <%c>\n", id, dest)
			case packet.FAILURE:
				fmt.Printf("handleRecv<%c> - FAILURE PACKET to <%c>\n", id, dest)
			default:
				fmt.Printf("handleRecv<%c> - <%s> to <%c>\n", id, packet.FmtBits(buffer[:n]), dest)
			}
		}
		if !corrupt {
			m.Lock()
			packets[dest] = append(packets[dest], buffer[:n])
			m.Unlock()
		}
	}
errored:
	c.Close()
	fmt.Printf("- Connection from <%c>\n", id)
	isClosed[id] = true
}

func main() {
	arguments := os.Args
	PORT := ""
	rand.Seed(time.Now().UnixNano())
	if len(arguments) == 1 {
		fmt.Println("Using default port of 4004\n________________\n")
		PORT = ":4004"
	} else {
		PORT = ":" + arguments[1]
	}

	l, err := net.Listen("tcp4", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go handleReceive(c)
	}
}
