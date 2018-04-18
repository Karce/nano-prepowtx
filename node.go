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
    "flag"
    "os"
    "math"
    "math/big"
    "time"
    "strconv"
    "crypto/rand"
)

var Wallet string
var NAccounts uint
var NTransactions uint64

var Accounts []string
var Balances []*big.Int
var Total *big.Int

// The number of transactions for every account.
var TransPerAccount uint

// The most recent block for every account.
var RecentHashes []string

// Blks will store all of the blocks that are created and signed offline.
// For each account, store a slice of blocks, in order.
// blks[account][block]
var Blks [][]string

var tCompute time.Duration

var Uid *big.Int

func main() {
    wallet := flag.String("wallet", "", "The wallet to sign/verify blocks")
    nAccounts := flag.Uint("n_accounts", 100, "The number of accounts to user/generate")
    nTransactions := flag.Uint64("n_trans", 30000, "The number of transactions to generate and send")
    flag.Parse()

    Wallet = *wallet
    NAccounts = *nAccounts
    NTransactions = *nTransactions

    fmt.Println("wallet:", Wallet)
    fmt.Println("n_accounts:", NAccounts)
    fmt.Println("n_trans:", NTransactions)

    TransPerAccount  = uint(NTransactions) / NAccounts

    if (Wallet == "") {
        fmt.Println("Error: No wallet was provided.")
        os.Exit(1)
    }
    // The total time to take to precompute a round of blocks.
    tCompute = 4 * time.Hour

    setupAccounts()

    max, nMax := findFunds()

    distributeFunds(max, nMax)

    // The network and work channel will deal with communications between the
    // network functions of the node with the computing side of the node.
    naw := make(chan string)

    // Create the Uid, if it doesn't exist.
    var maxUid *big.Int
    maxUid.SetString("1000000000", 10)
    Uid, _ = rand.Int(rand.Reader, maxUid)

    // Serv will handle all incoming connections.
    go serv()

    // peers["192.168.1.252"] = true
    // go request("192.168.1.252:9887")

    // From this point, coordinate communications between the network and the precomputing work.
    // A good number to attempt to hit on the network is 7,000 Transactions Per Second.
    // Attempt to adjust tCompute, network wide, to hit 7,000 TPS.
    // After every attack, broadcast the TPS to the network and gather TPS from other nodes.
    // For now, sync this number and coordinate attack.
    // For bootstrapping new nodes, they should attempt to connect to other nodes
    // and ask for the current tCompute (perhaps taking an average).
    for {
        // if time.Now.Hours() < i * tCompute
        // duration = (i * tCompute - time.Now) // This is the next time slot.
        cTime := time.Now().UTC()
        var nextTest time.Time
        var timeToTest time.Duration

        for i := 0.0; i * tCompute.Hours() <= 24.0; i++ {
            if (float64(cTime.Hour()) < i * tCompute.Hours()) {
                loc, _ := time.LoadLocation("UTC")
                nextTest = time.Date(cTime.Year(), cTime.Month(), cTime.Day(), int(i * tCompute.Hours()), 0, 0, 0, loc)
                timeToTest = nextTest.Sub(cTime)
                break
            }
        }
        fmt.Println("Next Attack Scheduled:", nextTest.Local().Format(time.UnixDate))

        go precomputeBlocks(naw, nMax)

        time.Sleep(timeToTest)
        fmt.Println("\n---Scheduled Time Reached---")

        // Halt the PoW and begin processing transactions.
        naw <- "halt"

        response := <-naw
        if (response != "halted") {
            // Some error
            os.Exit(1)
        }
        countMax, _ := strconv.ParseUint(<-naw, 10, 64)
        processBlocks(countMax)
    }
}

func setupAccounts() {
    // GET THE NUMBER OF ACCOUNTS FOR THE WALLET
    nWalletAccounts := uint(0)
    Accounts = AccountList()
    for i := uint(0); i < NAccounts; i++ {
        if (len(Accounts[i]) > 0) {
            nWalletAccounts++
        }
    }
    // GENERATE THE REMAINING ACCOUNTS
    if (nWalletAccounts < NAccounts) {
        for i := nWalletAccounts; i < NAccounts; i++ {
            GenerateAccount()
        }
    }
}

func findFunds() (*big.Int, uint) {
    // FIND FUNDS
    Total = big.NewInt(0)
    var fakeBalances = GetBalances()
    // Initialize the global balances for every account. This is required.
    Balances = make([]*big.Int, NAccounts)
    // Find the account with the most funds.
    max := big.NewInt(0)
    var nMax uint
    for i := uint(0); i < NAccounts; i++ {
        balance := big.NewInt(0)
        balance.SetString(fakeBalances[Accounts[i]].Balance, 10)
        Balances[i] = balance
        if (balance.Cmp(max) > 0) {
            max.Set(balance)
            nMax = i
        }
        fmt.Println("Account:", Accounts[i], "Balance:", balance)
        Total.Add(Total, balance)
    }
    fmt.Println("Total Balance:", Total)
    return max, nMax
}

func distributeFunds(max *big.Int, nMax uint) {
    // DISTRIBUTE FUNDS OR EXIT FOR INSUFFICIENT FUNDS
    // minimum is NTransactions
    minimum := big.NewInt(0)
    minimum.SetUint64(NTransactions)
    if (Total.Cmp(minimum) < 0) {
        fmt.Println("Insufficient funds: you need at least", minimum, "raw. You have", Total, "raw.")
        os.Exit(1)
    }

    // GET ALL PREVIOUS BLOCKS FOR THE ACCOUNTS
    // RecentHashes needs initialization. This is required.
    RecentHashes = make([]string, NAccounts)
    for i := uint(0); i < NAccounts; i++ {
        RecentHashes[i] = GetPreviousBlock(Accounts[i])
    }
    amount := big.NewInt(int64(TransPerAccount))

    Blks = make([][]string, NAccounts)
    for i := uint(0); i < NAccounts; i++ {
        Blks[i] = make([]string, TransPerAccount)
    }

    var ETA time.Duration
    var total time.Duration
    var count uint64

    if (max.Cmp(minimum) < 0) {
        // More complicated, compile account balances.
    } else {
        // SEARCH FOR ACCOUNTS THAT HAVE LESS THAN MINIMUM RAW
        for k, account := range Accounts {
            if (Balances[k].Cmp(amount) < 0) {
                fmt.Print("\rProcessing Account: ", k)
                fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")

                start := time.Now()
                // ADD MINIMUM BALANCE
                // TODO: Watch for timeouts here...
                deficit := Balances[k].Sub(amount, Balances[k])
                RecentHashes[nMax] = Send(Accounts[nMax], account, deficit.String())
                Balances[nMax].Sub(Balances[nMax], deficit)
                // RECEIVE THE BLOCK
                RecentHashes[k] = ReceiveBlock(account, RecentHashes[nMax])
                Balances[k].Set(amount)
                stop := time.Now()

                elapsed := stop.Sub(start)
                total += elapsed
                count++
                ETA = time.Duration((uint64(total) / count) * uint64(NAccounts - uint(k)))
             }
        }
        fmt.Println()
    }
    fmt.Println("---Finished Setting Up Accounts---")
}

func precomputeBlocks(naw chan string, nMax uint) {
    // ITERATE OVER EACH ACCOUNT
    // CREATE BLOCKS
    var ETA time.Duration
    var total time.Duration
    var count uint64
    fmt.Println("---Begin Precomputing PoW (Send Blocks)---")
    amount := big.NewInt(1)
    for i := uint(0); i < TransPerAccount; i++ {
        for k, account := range Accounts {
            select {
            case msg := <-naw:
                if (msg == "halt") {
                    naw <- "halted"
                    fmt.Println("---Halting Precomputation---")
                    naw <- strconv.FormatUint(count, 10)
                    return
                }
            default:
            }
            fmt.Print("\rBlock: ", count, "/", NTransactions, ", ", math.Floor((float64(count) / float64(NTransactions) * 1000)) / 10, "%")
            fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")
            start := time.Now()
            RecentHashes[k], Blks[k][i] = CreateSendBlock(account, Accounts[nMax], (Balances[k].Sub(Balances[k], amount)).String(), amount.String(), RecentHashes[k])
            stop := time.Now()
            elapsed := stop.Sub(start)
            total += elapsed
            count++
            ETA = time.Duration((uint64(total) / count) * (NTransactions - count))
        }
    }
    fmt.Println()
    fmt.Println("---Finished Precomputing Blocks---")
}

func processBlocks(max uint64) {
    // PROCESS BLOCKS
    var ETA time.Duration
    var total time.Duration
    var count uint64
    fmt.Println("---Begin Stress Test (Publishing Blocks)---")
    for i := uint(0); i < TransPerAccount; i++ {
        for k, _ := range Accounts {
            if (count > max) {
                fmt.Println("\n---Finished Processing Blocks---")
                return
            }
            fmt.Print("\rBlock: ", count, "/", max, ", ", math.Floor((float64(count) / float64(max) * 1000)) / 10, "%")
            fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")
            start := time.Now()
            RecentHashes[k] = ProcessBlock(Blks[k][i])
            stop := time.Now()
            elapsed := stop.Sub(start)
            total += elapsed
            count++
            ETA = time.Duration((uint64(total) / count) * (max - count))
        }
    }
    fmt.Println()
}

func receiveAllPending() {
    for _, account := range Accounts {
        // Later, do not just blindly call this but filter it
        // by only calling when there are actually pending blocks
        // on the account.
        receivePending(account)
    }

}

func receivePending(account string) {
    // GET ALL PENDING SOURCE BLOCKS FOR ACCOUNT
    fmt.Println("Receiving Blocks for Account: ", account)
    var total time.Duration
    var count uint64

    var hash string = GetPreviousBlock(account)
    var pending []string = GetPendingBlocks(account, "100")
    for len(pending) > 0 {
        for i := 0; i < len(pending); i++ {
            start := time.Now()
            hash = receivePendingBlock(account, pending[i], hash)
            stop := time.Now()
            elapsed := stop.Sub(start)
            total += elapsed
            count++
            average := time.Duration(uint64(total) / count)
            fmt.Print("\rBlock: ", i + 1, "/", len(pending), ", Time/Receive (TPS): ", average.String())
        }
        pending = GetPendingBlocks(account, "100")
    }
}

func receivePendingBlock(account, source, previous string) (string) {
    // var block string
    // _, block = CreateReceiveBlock(account, source, previous)
    // hash := ProcessBlock(block)
    hash := ReceiveBlock(account, source)
    return hash
}

// Version - The version number of the node.
// Uid - A unique identifier used to identify nodes.
// Action - The action of the request
type Header struct {
    Version uint
    Uid *big.Int
    Action string
}

var pLock sync.Mutex

var peers = make(map[string]bool)
var myAddress string

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

func request(address string) {
    // The purpose of this function is to make requests to other nodes on the network.
    if (address == myAddress) {
        // Don't make requests to ourselves.
        return
    }

    var example Header
    example.Version = 2
    example.Uid = Uid
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
        if (h.Uid.Cmp(Uid) == 0) {
            fmt.Println("Found myself")
            myAddress = conn.RemoteAddr().String()
        }
        relayPeers(enc)
        fmt.Println("Relayed Peers")
    }
    fmt.Println("...Terminating Connection...")
    err = conn.Close()
    if err != nil {
	    // handle error
        fmt.Println(err)
	}
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

