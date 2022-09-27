package packet

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"
	"time"
)

var verbose bool = false

// "packet" from pseudo-client/server
// | dest | src  | seq    | flags | padding | * size | data     | checksum |
// | 0x00 | 0x00 | 0x0000 | 00000 | 00...1  | 0x0000 | 0x...    | 0x0000   |
// | i8   | i8   | i16    | SAIFD | 3/11 b  | i16    | max size | i16      |
//
// size = amount of bits in datasection, this only exists if S is 1
//	it allows the server to expect the amount of data coming in
//	(seq * size) - if S is 0, then size doesnt exist and is instead just data
//
// padding = upto 15 bits, parser needs to not read size/data
//	till first 1 is spotted after flags - this ensures the checksum is valid
//
// ___ * = explanation of what it means if flag is 1 ___
// S = start of transmission
//	sequence will hold the total amount of incoming packets
//  and size will hold the amount of bytes in each transactions data section
//  - note, that if S is true, then current implementation of wrapper functions expects
//  - data to be empty, however theres nothing to limit another wrapper function having both
//
// A = accepted start of transmission
//	sequence & size should hold same value as when S flag was received
//
// I = ignoring you, i can't talk to you right now
//	response to S flag packets if server/client exists/online but not seeking that data
//
// F = failed to receive all of the data
//	e.g. some sequence is missing
//	implicit prompt for restarting transmission from S flag
//
// D = done, i received all sequences :D

const (
	START   byte = 0b10000000
	ACCEPT       = 0b01000000
	IGNORE       = 0b00100000
	FAILURE      = 0b00010000
	DONE         = 0b00001000
	EMPTY        = 0b00000000
)

// https://gist.github.com/chiro-hiro/2674626cebbcb5a676355b7aaac4972d
func i16tob(val uint16) []byte {
	r := make([]byte, 2)
	for i := uint16(0); i < 2; i++ {
		r[i] = byte((val >> (8 * i)) & 0xff)
	}
	return r
}

func btoi16(val []byte) uint16 {
	r := uint16(0)
	for i := uint16(0); i < 2; i++ {
		r |= uint16(val[i]) << (8 * i)
	}
	return r
}

// ------------------------

// https://stackoverflow.com/a/31632586
func FmtBits(data []byte) []byte {
	var buf bytes.Buffer
	for _, b := range data {
		fmt.Fprintf(&buf, "%08b ", b)
	}
	if buf.Len() > 0 {
		buf.Truncate(buf.Len() - 1) // To remove extra space
	}
	return buf.Bytes()
}

func calculateChecksum(data []byte) []byte {
	var checksum uint16 = 0
	for i := 0; i <= (len(data) - 2); i += 2 {
		checksum += btoi16(data[i : i+2])
	}
	checksum ^= 0xffff
	return i16tob(checksum)
}

func verifyChecksum(data []byte) bool {
	var checksum uint16 = 0
	for i := 0; i <= (len(data) - 2); i += 2 {
		checksum += btoi16(data[i : i+2])
	}
	if checksum == math.MaxUint16 {
		return true
	} else {
		return false
	}
}

func Encode(dest byte, src byte, seq uint16, flag byte, size uint16, data []byte) (buffer []byte) {
	buffer = make([]byte, 0)

	// ---------- ensuring that final buffer is 2byte padded (technically everything but bytes and slices of bytes can be ignored)
	// byte, src, seq, checksum
	length := 1 + 1 + 2 + 2
	if (flag & (START | ACCEPT | DONE)) > 0 {
		// size
		length += 2
	}
	length += len(data)
	// too much data transmitted at once
	if len(data) > 0xffff {
		return
	}
	flags_and_padding := []byte{flag}
	if length%2 == 0 {
		flags_and_padding = append(flags_and_padding, 0b00000000)
	}
	flags_and_padding[len(flags_and_padding)-1] |= 0b00000001

	buffer = append(buffer, dest, src)
	seq_bytes := i16tob(seq)
	buffer = append(buffer, seq_bytes...)
	buffer = append(buffer, flags_and_padding...)
	if (flag & (START | ACCEPT | DONE)) > 0 {
		size_bytes := i16tob(size)
		buffer = append(buffer, size_bytes...)
	}
	buffer = append(buffer, data...)

	buffer = append(buffer, calculateChecksum(buffer)...)
	return
}

func Decode(raw []byte) (corrupt bool, valid bool, dest byte, src byte, seq uint16, flag byte, size uint16, data []byte) {
	valid = verifyChecksum(raw)
	// minimum packet length
	if len(raw) < 7 {
		if verbose {
			fmt.Println("D1-", len(raw))
		}
		corrupt = true
		return
	}
	dest = raw[0]
	src = raw[1]
	seq = btoi16(raw[2:4])
	flag = raw[4]
	offset := 5
	if flag&0b00000001 == 1 {
		flag &= 0b11111110
	} else {
		offset += 1
	}
	// these flags have no meaning, it should never be true
	if flag&0b00000110 > 0 {
		valid = false
		return
	}
	if offset+2 > len(raw) {
		if verbose {
			fmt.Println("D2-", offset+2, len(raw))
		}
		corrupt = true
		return
	}
	if (flag & (START | ACCEPT | DONE)) > 0 {
		size = btoi16(raw[offset : offset+2])
		offset += 2
	}
	if offset < len(raw)-2 {
		data = raw[offset : len(raw)-2]
	}
	return
}

// Send is a wrapper of net.Conn.Read/write - making sure that data is verified by receiver
// it can be thought of as a localized state machine (modelling simplified TCP) for a specific data transfer.
// window is how many bytes (of data) it is allowed to send per 'packet'
// tolerance is how many times it will allow restarting communication process before it fails
func Send(c net.Conn, src byte, dest byte, data []byte, window uint16, tolerance uint16) (e error) {
	// 65543 is max size of our 'packet'
	buffer := make([]byte, 65543)
	var attempts uint16 = 0

	// if its not possible to transmit all of the data
	if (int(window) * int(math.MaxUint16)) < len(data) {
		e = errors.New("Window needs to be larger to allow transmit of data")
		return
	}

	var seqs uint16 = 0
	if window != 0 {
		seqs = uint16(len(data) / int(window))
		if len(data)%int(window) != 0 {
			seqs++
		}
	}

	query := Encode(dest, src, seqs, START, window, []byte{})
	if verbose {
		fmt.Printf("Send(1): <%s>\n", FmtBits(query))
	}
await_confirm:
	if attempts > tolerance {
		e = errors.New("Attempts exceeded set tolerance")
		return
	}
	attempts++

	c.Write(query)
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := c.Read(buffer)
	// remove the deadline
	c.SetReadDeadline(time.Time{})
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Timed out waiting for initial response")
			goto await_confirm
		}
		e = err
		return
	}

	if verbose {
		fmt.Printf("Send(2): <%s>\n", FmtBits(buffer[:n]))
	}

	corrupt, valid, destR, srcR, seqR, flag, size, _ := Decode(buffer[:n])
	if corrupt || !valid {
		if verbose {
			fmt.Printf("Send(2A): Failed...\n")
		}
		goto await_confirm
	}
	if src != destR || dest != srcR || seqs != seqR || window != size {
		if verbose {
			fmt.Printf("Send(2B): Failed...\n")
			fmt.Printf("src: %v != destR %v, dest: %v != srcR %v\n", src, destR, dest, srcR)
			fmt.Printf("seqs: %v != seqR %v, window: %v != size %v\n", seqs, seqR, window, size)
		}
		goto await_confirm
	}
	if flag&IGNORE > 0 {
		if verbose {
			fmt.Printf("Send(2C): Failed...\n")
		}
		e = errors.New("Server is not accepting communication right now")
		return
	} else if flag&ACCEPT == 0 {
		if verbose {
			fmt.Printf("Send(2D): Failed...\n")
		}
		goto await_confirm
	}

	data_to_send := len(data)
	data_sent, slice_offset := 0, 0
	var seq uint16 = 0
	// note how it diverges from good TCP practice here, since it doesnt ACK
	// individual "packets" in split up data, but instead just does it once at the end
	// this means that if the server already way at the start knows its receiving junk data
	// this will still finish sending all of its packets - before receiving the info from server
	for data_sent < data_to_send {
		slice_offset = data_sent + int(window)
		if slice_offset > data_to_send {
			slice_offset = data_to_send
		}
		data_packet := Encode(dest, src, seq, EMPTY, 0, data[data_sent:slice_offset])
		if verbose {
			fmt.Printf("Send(3-%v): <%s>\n", seq, FmtBits(data_packet))
			fmt.Printf("Send(3+%v): <%s>\n", seq, FmtBits(data[data_sent:slice_offset]))
		}
		// it might deceptively seem like we can timeout waiting here forever, but we actually cant
		// due to communication going through forwarder, it will always be able to send
		c.Write(data_packet)
		data_sent += int(window)
		seq++
	}
	if data_sent > 0 {
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := c.Read(buffer)
		c.SetReadDeadline(time.Time{})
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("DONE packet not received, assuming transmission failed - restarting.")
				goto await_confirm
			}
			e = err
			return
		}

		corrupt, valid, destR, srcR, seqR, flag, size, _ = Decode(buffer[:n])
		if verbose {
			fmt.Printf("Send(4): <%s>\n", FmtBits(buffer[:n]))
		}
		if corrupt || !valid {
			if verbose {
				fmt.Printf("Send(4A): Failed...\n")
			}
			goto await_confirm
		}
		if src != destR || dest != srcR || seqs != seqR || window != size {
			if verbose {
				fmt.Printf("Send(4B): Failed...\n")
				fmt.Printf("src: %v != destR %v, dest: %v != srcR %v\n", src, destR, dest, srcR)
				fmt.Printf("seqs: %v != seqR %v, window: %v != size %v\n", seqs, seqR, window, size)
			}
			goto await_confirm
		}
		if flag&FAILURE > 0 {
			if verbose {
				fmt.Printf("Send(4C): Failed...\n")
			}
			goto await_confirm
		}
	}
	// if the response from the server doesn't have the flag DONE, even though we're not sending
	// more packets - then just try again, obviously this isn't ideal either - since that means if the
	// last packet from server that just acknowledges it got all the data is corrupt, then the entire process
	// starts over.
	// to clarify, if all the previous logic has gotten us here, there is no logical flow that could lead to it
	// not needing to be DONE from the server side - however, since the checksum said it was valid, and the flag is not
	// what we expect, the network is proven unstable enough to change multiple bits - so we can only start over
	if flag&DONE == 0 {
		goto await_confirm
	}
	return
}

// Recv is the complimentary wrapper to Send - they need to be used in combination
func Recv(c net.Conn, src byte, wait uint8) (data []byte, srcR byte, e error) {
	// 65543 is max size of our 'packet'
	buffer := make([]byte, 65543)

await_start:
	if wait > 0 {
		c.SetReadDeadline(time.Now().Add(time.Duration(wait) * time.Second))
	}
	n, err := c.Read(buffer)
	c.SetReadDeadline(time.Time{})
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Timed out waiting for START-packet")
		}
		e = err
		return
	}

	corrupt, valid, destR, srcR, seqR, flag, size, data := Decode(buffer[:n])
	if verbose {
		fmt.Printf("Recv(1): <%s>\n", FmtBits(buffer[:n]))
	}
	if corrupt || !valid {
		fmt.Printf("Received an invalid packet, waiting for another response\n")
		goto await_start
	}
	if src != destR {
		if verbose {
			fmt.Printf("Recv(1B): Failed here...\n")
		}
		goto await_start
	}
	if flag&START == 0 {
		if verbose {
			fmt.Printf("Recv(1C): Failed here...\n")
		}
		goto await_start
	}
	if seqR == 0 || size == 0 {
		accept_packet := Encode(srcR, src, seqR, ACCEPT|DONE, size, []byte{})
		c.Write(accept_packet)
		return
	}

	accept_packet := Encode(srcR, src, seqR, ACCEPT, size, []byte{})
	c.Write(accept_packet)
	if verbose {
		fmt.Printf("Recv(2): <%s>\n", FmtBits(accept_packet))
	}

	received := make([][]byte, seqR)

	var seqs uint16 = 0
	// note how it diverges from good TCP practice here, since it doesnt ACK
	// individual "packets" in split up data, but instead just does it once at the end
	for seqs < seqR {
		msg_buffer := make([]byte, 65543)
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := c.Read(msg_buffer)
		c.SetReadDeadline(time.Time{})
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("Missing data packets, sending failure-packet and awaiting START")
				fail_packet := Encode(srcR, src, seqR, FAILURE, size, []byte{})
				if verbose {
					fmt.Printf("Recv(3A-%v): <%s>\n", seqs, FmtBits(fail_packet))
				}
				c.Write(fail_packet)
				goto await_start
			}
			return
		}

		corrupt, valid, _, _, seqTmp, flagTmp, _, dataTmp := Decode(msg_buffer[:n])
		if verbose {
			fmt.Printf("Recv(3-%v): <%s>\n", seqs, FmtBits(msg_buffer[:n]))
		}
		if corrupt || !valid || flagTmp != EMPTY {
			fail_packet := Encode(srcR, src, seqR, FAILURE, size, []byte{})
			if verbose {
				fmt.Printf("Recv(3A-%v): <%s>\n", seqs, FmtBits(fail_packet))
			}
			c.Write(fail_packet)
			goto await_start
		}
		if len(received[seqTmp]) != 0 {
			if verbose {
				fmt.Printf("Recv(3B-%v): Failed...\n", seqs)
			}
			goto await_start
		} else {
			received[seqTmp] = dataTmp
			if verbose {
				fmt.Printf("Recv(3+%v): <%s>\n", seqs, FmtBits(dataTmp))
			}
		}
		seqs++
	}
	data = []byte{}
	for i := 0; i < int(seqs); i++ {
		data = append(data, received[i]...)
	}
	done_packet := Encode(srcR, src, seqR, DONE, size, []byte{})
	if verbose {
		fmt.Printf("Recv(4): <%s>\n", FmtBits(done_packet))
	}

	c.Write(done_packet)
	return
}
