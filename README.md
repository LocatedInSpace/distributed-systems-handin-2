# a) What are packets in my implementation . . .
I've recreated a simple TCP-inspired send & receive (on top of Golangs net-package) - that means, all of the communication happens in bytes.

As seen in `packet/packet.go`, this is how I've laid out a packet.
```go
// "packet" from pseudo-client/server
// | dest | src  | seq    | flags | padding | * size | data     | checksum |
// | 0x00 | 0x00 | 0x0000 | 00000 | 00...1  | 0x0000 | 0x...    | 0x0000   |
// | i8   | i8   | i16    | SAIFD | 3/11 b  | i16    | max size | i16      |
```
The full explanation of all the flags can be seen in abovementioned file, however the idea is that it uses flags to synchronize at what part of communication it is on.

`S(tart)` prompts `A(ccept)`, ending with `D(one)` once all data has been transferred. Based on the values in `seq` and `size` when `S(tart)`-packet is transmitted, the receiver knows how many packets are about to be sent (so when it has received that amount of packets with a valid checksum, it will respond with `D(one)`).

That does mean that unlike real TCP, based on parameters the wrapper-functions (`packet.Recv()` & `packet.Send()`) receives - it will not acknowledge *on* every packet (but it will still acknowledge the stream that they were a part of).

>![Good TCP-model](images/1to1packetdone.png)
Image of communication (see forwarder output on the left), where every packet gets acknowledged and responded to by the other end

>![Less than ideal TCP-behaviour](images/streampacketsdone.png)
Image of communication (see forwarder output on the left), where it waits till stream of packets have come in to determine whether or not communication succeeded - ignore the noise in server (topright) & client (bottomright), verbose output is needed when splitting up data to several packets. See note way at bottom of README.md


# Addendum
##  How to run
Inside of `pseudo_server.go` & `pseudo_client.go`, there are two example usages of my TCP-model - it is important that they match, so that if `pseudo_server.go` has been switched over to the pinging example, that `pseudo_client.go` also has.

 1. Comment out the example you wish to run in each file - or let it stay default.

 2. Run each of the files in separate terminals - the order of `forwarder.go` is important, it needs to be run first, the same is not true for `pseudo_client.go` and `pseudo_server.go`:

    ```console
    $ go run forwarder.go
    $ go run pseudo_server.go
    $ go run pseudo_client.go
    ```
## Changing network stability etc.
`forwarder.go` has all of the variables for changing network behaviour, these can be found at the top of the file:

    ```go
    // network has bad jitter
    var jitter bool = false

    // percentage for packet to be dropped, ignored
    var drop_packet int = 0

    // percentage for packet to have a bit flipped
    var flip_bit int = 0
    ```

>It is worth noting, that the effects of jitter wont be seen, unless the `window`-parameter in `packet.Send()` is less than the size of the data to be sent.
See line 49/50 in `pseudo_client.go`


**OBS.** with a low value of `window`-parameter - the chance of `packet.Recv()` & `packet.Send()` failing is very high UNLESS `verbose = true` in `packet.go`

Yes, I am aware that this is not ideal behaviour - but for some reason (maybe slowing it down?), it fails **measurably** less when it prints stuff... even though it has the exact same controlflow
