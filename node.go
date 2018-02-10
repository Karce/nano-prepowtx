/*
 * Copyright (C) 2018 Keaton Bruce
 *
 * This file is part of NanoBots.
 *
 * NanoBots is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * NanoBots is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with NanoBots. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
    "fmt"
    "net"
    "strings"
    "encoding/gob"
    "sync"
    "log"
)

func main() {
    peers["192.168.1.252"] = true
    TestRPC()
    serv()
}

type Header struct {
    Id uint
    Action string
}

var pLock sync.Mutex

var peers = make(map[string]bool)

func serv() {
    // The purpose of this function is to listen for new connections concurrently.
    ln, err := net.Listen("tcp", ":9887")
    if err != nil {
	    // handle error
        fmt.Println(err)
    }
    go request("192.168.1.252:9887")
    for {
	    conn, err := ln.Accept()
	    if err != nil {
		    // handle error
            fmt.Println(err)
	    }
	    go handleConnection(conn)
    }
}

func request(address string) {
    // The purpose of this function is to make requests to other nodes on the network.
    var example Header
    example.Id = 4294961111
    example.Action = "get_peers"

    fmt.Println("Connecting to:", address)
    conn, err := net.Dial("tcp", address)
    if err != nil {
	    // handle error
        fmt.Println(err)
    }

    enc := gob.NewEncoder(conn)
    err = enc.Encode(example)
    if err != nil {
        log.Fatal("encode error:", err)
    }

    dec := gob.NewDecoder(conn)
    fmt.Println("Receiving Peers from:", conn.RemoteAddr()) 
    receivePeers(dec)
}

func handleConnection(conn net.Conn) {
    // This function handles requests.
    // It will read the message header, determine the action to take, 
    // and then return that information back to the peer.

    fmt.Printf("...Connection Established to %s...\n", conn.RemoteAddr())
    // Add the peer to the list of known Peers.
    pLock.Lock()
    peers[strings.Split(conn.RemoteAddr().String(), ":")[0]] = true
    pLock.Unlock()

    dec := gob.NewDecoder(conn)
    // Decode (receive) the value.
    var h Header
    err := dec.Decode(&h)
    if err != nil {
        log.Fatal("decode error:", err)
    }

    fmt.Println(h.Action)
    enc := gob.NewEncoder(conn)
    if (h.Action == "get_peers") {
        relayPeers(enc)
        fmt.Println("Relayed Peers")
    }
    fmt.Println("...Terminating Connection...")
    err = conn.Close()
    if err != nil {
	    // handle error
        fmt.Println(err)
	}
    return
}

func relayPoW() {
    // Send the number of precached PoW's.
}



func relayPeers(enc *gob.Encoder) {
    pLock.Lock()
    err := enc.Encode(peers)
    pLock.Unlock()

    if err != nil {
        log.Fatal("encode error:", err)
    }
}

func receivePeers(dec *gob.Decoder) {
    var pMap map[string]bool
    err := dec.Decode(&pMap)
    if err != nil {
        log.Fatal("decode error:", err)
    }

    pLock.Lock()
    for k := range pMap {
        _, ok := peers[k]
        if !ok {
            // Create new connections here.
            peers[k] = true
            go request(k + ":9887")
        }
    }
    pLock.Unlock()
}

