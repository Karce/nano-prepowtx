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
)

/*
 * This file contains two major parts.
 *
 * 1. The ability to precompute blocks by offline signing.
 *     - This is acheived by calling the nano-node RPC interface.
 *     - These blocks are then stored and saved for later use.
 *
 * 2. The capability to synchronize attacks with other P2P nodes.
 *     - Bootstrapping is required and a DHT is utilized.
 *     - Trust is, right now, important for the network.
 */

var Wallet string
var NAccounts uint
// var NTransactions uint64

var Accounts []string
var Balances []*big.Int
var Total *big.Int

// The default number of transactions for every account.
var DefaultTPA uint

// The most recent block for every account.
var Hashes [][]string

// Blks will store all of the blocks that are created and signed offline.
// For each account, store a slice of blocks, in order.
// blks[account][block]
var Blks [][]string

var tCompute time.Duration
var LastPoWMax uint64

// var Uid *big.Int

func main() {
    wallet := flag.String("wallet", "", "The wallet to sign/verify blocks")
    nAccounts := flag.Uint("n_accounts", 100, "The number of accounts to user/generate")
    // nTransactions := flag.Uint64("n_trans", 30000, "The number of transactions to generate and send")
    flag.Parse()

    Wallet = *wallet
    NAccounts = *nAccounts
    // NTransactions = *nTransactions

    fmt.Println("wallet:", Wallet)
    // fmt.Println("n_accounts:", NAccounts)
    // fmt.Println("n_trans:", NTransactions)

    // This is the starting Transactions Per Account.
    // Usefull for allocating space to the initial Blks slice.
    DefaultTPA  = uint(1000) / NAccounts

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

    // Serv will handle all incoming connections.
    // go serv()

    // peers["127.0.0.1"] = true
    // go request("127.0.0.1", "get_peers")

    // From this point, coordinate communications between the network and the precomputing work.
    // A good number to attempt to hit on the network is 7,000 Transactions Per Second.
    // Attempt to adjust tCompute, network wide, to hit 7,000 TPS.
    // After every attack, broadcast the TPS to the network and gather TPS from other nodes.
    // For now, sync this number and coordinate attack.
    // For bootstrapping new nodes, they should attempt to connect to other nodes
    // and ask for the current tCompute (perhaps taking an average).
    for count := 0;;count++ {
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

        // Alternate between sending blocks and receiving blocks based on the count.
        go precomputeBlocks(naw, nMax, count, LastPoWMax, nextTest)

        time.Sleep(timeToTest)
        fmt.Println("\n---Scheduled Time Reached---")

        // Halt the PoW and begin processing transactions.
        naw <- "halt"

        response := <-naw
        if (response != "halted") {
            // Some error
            os.Exit(1)
        }
        LastPoWMax, _ = strconv.ParseUint(<-naw, 10, 64)
        processBlocks(LastPoWMax)
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
    minimum.SetUint64(100000)
    if (Total.Cmp(minimum) < 0) {
        fmt.Println("Insufficient funds: you need at least", minimum, "raw. You have", Total, "raw.")
        os.Exit(1)
    }

    amount := big.NewInt(int64(DefaultTPA))

    // GET ALL PREVIOUS BLOCKS FOR THE ACCOUNTS
    // RecentHashes needs initialization. This is required.

    Hashes = make([][]string, NAccounts)
    Blks = make([][]string, NAccounts)
    for i := uint(0); i < NAccounts; i++ {
        Blks[i] = make([]string, DefaultTPA)
        Hashes[i] = make([]string, DefaultTPA)
        // RecentHashes[i] = GetPreviousBlock(Accounts[i])
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
                Hashes[nMax][0] = Send(Accounts[nMax], account, deficit.String())
                Balances[nMax].Sub(Balances[nMax], deficit)
                // RECEIVE THE BLOCK
                Hashes[k][0] = ReceiveBlock(account, Hashes[nMax][0])
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

func precomputeBlocks(naw chan string, nMax uint, iteration int, maximum uint64, nextTest time.Time) {
    // ITERATE OVER EACH ACCOUNT
    // CREATE BLOCKS
    var ETA time.Duration
    var total time.Duration
    var count uint64
    var estimate uint64
    fmt.Println("---Begin Precomputing PoW (Send or Receive Blocks)---")
    amount := big.NewInt(1)
    // Continue to produce blocks until the scheduled attack time.
    // Estimate how many blocks that will be.
    for i := uint(0);; i++ {
        // Accounts[i % NAccounts] = the account to process at the moment.
        k := i % NAccounts
        // iter is the 'round' for each account.
        iter := i / NAccounts

        if i > uint(len(Blks[k])) * NAccounts {
            fmt.Println("Copying block data, new size:", len(Blks[k]) + 1 * 2)
            // Increase the length of Blks and Hashes.
            for j := uint(0); j < NAccounts; j++ {
                // Create the new space.
                b := make([]string, len(Blks[k]) + 1 * 2)
                h := make([]string, len(Hashes[k]) + 1 * 2)
                // Copy the data into the bigger slices.
                copy(b, Blks[j])
                copy(h, Hashes[j])
                // Reassign the slices to the bigger slices.
                Blks[j] = b
                Hashes[j] = h
            }
        }

        fmt.Print("\rBlock: ", count, "/", estimate, ", ", math.Floor((float64(count) / float64(estimate) * 1000)) / 10, "%")
        fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")
        start := time.Now()
        if iteration % 2 == 0 {
            var recentHash uint = 0
            if iter > 0 {
                recentHash = iter - 1
            }
            Hashes[k][iter], Blks[k][iter] = CreateSendBlock(Accounts[k], Accounts[(i + 1) % NAccounts], Balances[k].String(), amount.String(), Hashes[k][recentHash])
            Balances[k].Sub(Balances[k], amount)
        } else {
            if uint64(i) > maximum {
                fmt.Println("---Reached the maximum amount of blocks to receive---")
                return
            }
            // The number should be so large that it will resolve to empty string.
            // The transition between creating send blocks and creating receive blocks
            // requires a lookup to know exactly what the most recent hash was for each account.
            // This is because the interupted process before this will probably be incomplete.
            var recentHash uint = uint(len(Hashes[k]))
            if iter > 0 {
                recentHash = iter - 1
            }
            Hashes[k][iter], Blks[k][iter] = CreateReceiveBlock(Accounts[k], Hashes[(i - 1) % NAccounts][iter], Hashes[k][recentHash])
            Balances[k].Add(Balances[k], amount)
        }
        stop := time.Now()
        elapsed := stop.Sub(start)
        total += elapsed
        count++
        estimate = uint64(i) + uint64((time.Until(nextTest) / time.Duration(uint64(total) / count)))
        ETA = time.Duration((uint64(total) / count) * (estimate - count))

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
    for i := uint(0); i < DefaultTPA; i++ {
        for k, _ := range Accounts {
            if (count > max) {
                fmt.Println("\n---Finished Processing Blocks---")
                return
            }
            fmt.Print("\rBlock: ", count, "/", max, ", ", math.Floor((float64(count) / float64(max) * 1000)) / 10, "%")
            fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")
            start := time.Now()
            Hashes[k][count] = ProcessBlock(Blks[k][i])
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
    // Uid *big.Int
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

func request(address, action string) {
    // The purpose of this function is to make requests to other nodes on the network.
    if (address == myAddress) {
        // Don't make requests to ourselves.
        return
    }
    address += ":9887"

    var request Header
    request.Version = 3
    request.Action = action

    fmt.Println("Connecting to:", address)
    conn, err := net.Dial("tcp", address)
    if err != nil {
	    // handle error
        fmt.Println(err)
    }
    if (myAddress == "") {
        myAddress = strings.Split(conn.LocalAddr().String(), ":")[0]
        fmt.Println("My Address:", myAddress)
    }

    enc := gob.NewEncoder(conn)
    err = enc.Encode(request)
    if err != nil {
        log.Fatal("encode error:", err)
    }
    switch action {
    case "get_peers":
        dec := gob.NewDecoder(conn)
        fmt.Println("Receiving Peers from:", conn.RemoteAddr())
        receivePeers(dec)
    case "relay_pow":
    }
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
}

/*
func relayPoW() {
    // Send the number of precached PoW's.
    for k, v := range peers {
        // If haven't received the most recent PoWs from peer
        if (!PoWs[k]) {
            go request(k, "relay_pow")
        }
    }
}
*/

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
            go request(k, "get_peers")
        }
    }
    pLock.Unlock()
}

