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
    "encoding/gob"
    "log"
    "math/big"
    "time"
)

var tCompute time.Duration
var NextTest time.Time

// Version - The version number of the node.
// Uid - A unique identifier used to identify nodes.
// Action - The action of the request
type Header struct {
    Version uint
    Uid *big.Int
    Action string
}

func main() {
    go serv()
    // The total time to take to precompute a round of blocks.
    tCompute = 4 * time.Hour

    // Time loop, for now continue to adjust the tCompute as clients connect.
    for {
        // if time.Now.Hours() < i * tCompute
        // duration = (i * tCompute - time.Now) // This is the next time slot.
        cTime := time.Now().UTC()
        for i := 0.0; i * tCompute.Hours() <= 24.0; i++ {
            if (float64(cTime.Hour()) < i * tCompute.Hours()) {
                loc, _ := time.LoadLocation("UTC")
                NextTest = time.Date(cTime.Year(), cTime.Month(), cTime.Day(), int(i * tCompute.Hours()), 0, 0, 0, loc)
                break
            }
        }
        fmt.Println("Next Attack Scheduled:", NextTest.Local().Format(time.UnixDate))
        waitTime := time.Until(NextTest)
        time.Sleep(waitTime)
    }
}

func serv() {
    // The purpose of this function is to listen for new connections concurrently.
    ln, err := net.Listen("tcp", ":9887")
    if err != nil {
	    // handle error
        fmt.Println(err)
    }
    for {
	    conn, err := ln.Accept()
	    if err != nil {
		    // handle error
            fmt.Println(err)
	    }
	    go handleConnection(conn)
    }
}

func handleConnection(conn net.Conn) {
    // This function handles requests.
    // It will read the message header, determine the action to take, 
    // and then return that information back to the peer.

    fmt.Printf("...Connection Established to %s...\n", conn.RemoteAddr())
    // Add the peer to the list of known Peers.
    // pLock.Lock()
    // peers[strings.Split(conn.RemoteAddr().String(), ":")[0]] = true
    // pLock.Unlock()

    dec := gob.NewDecoder(conn)
    // Decode (receive) the value.
    var h Header
    err := dec.Decode(&h)
    if err != nil {
        log.Fatal("decode error:", err)
    }

    fmt.Println(h.Action, h.Version, h.Uid.String())
    enc := gob.NewEncoder(conn)
    if (h.Action == "relay_tps") {
        var tps float64
        err = dec.Decode(&tps)
        if err != nil {
            log.Fatal("decode error:", err)
        }
        fmt.Println("TPS:", tps)
        enc.Encode(NextTest)
    }
    fmt.Println("...Terminating Connection...")
    err = conn.Close()
    if err != nil {
	    // handle error
        fmt.Println(err)
	}
}
