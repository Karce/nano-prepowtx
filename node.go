/*
 * Copyright (C) 2018 Keaton Bruce
 *
 * This file is part of nano-prepowtx.
 *
 * nano-prepowtx is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * nano-prepowtx is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with nano-prepowtx. If not, see <http://www.gnu.org/licenses/>.
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
var NAccounts uint64
// var NTransactions uint64

var Accounts []string
var Balances []*big.Int
var Total *big.Int

// The default number of transactions for every account.
var DefaultTPA uint64

// The most recent block for every account.
var Hashes [][]string

// Blks will store all of the blocks that are created and signed offline.
// For each account, store a slice of blocks, in order.
// blks[account][block]
var Blks [][]string

// The time interval to the next attack measured in seconds.
var tCompute int64
var LastPoWMax uint64

// var Uid *big.Int

func main() {
    wallet := flag.String("wallet", "", "The wallet to sign/verify blocks")
    nAccounts := flag.Uint64("n_accounts", 100, "The number of accounts to user/generate")
    flag.Parse()

    Wallet = *wallet
    NAccounts = *nAccounts

    fmt.Println("wallet:", Wallet)
    // fmt.Println("n_accounts:", NAccounts)
    // fmt.Println("n_trans:", NTransactions)

    // This is the starting Transactions Per Account.
    // Useful for allocating space to the initial Blks slice.
    DefaultTPA  = uint64(1000) / NAccounts

    if (Wallet == "") {
        fmt.Println("Error: No wallet was provided.")
        os.Exit(1)
    }

    // The total time to take to precompute a round of blocks in minutes.
    tCompute = int64((time.Duration(5) * time.Minute) / time.Second)

	fmt.Println("tCompute in seconds:", tCompute)

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
    for count := int64(0);;count++ {

		var cTime time.Time = time.Now()
        var timePastTest time.Duration = time.Duration(cTime.Unix() % tCompute) * time.Second
		var nextTest time.Time = cTime.Add((time.Duration(tCompute) * time.Second) - timePastTest)

        fmt.Println("Next Attack Scheduled:", nextTest.Format(time.UnixDate))

        // Alternate between sending blocks and receiving blocks based on the count.
        go precomputeBlocks(naw, nMax, count, LastPoWMax, nextTest)

		select {
		case message := <-naw:
			if (message != "finished") {
				// Some error
				os.Exit(1)
			}
		case <-time.After(time.Until(nextTest)):
			// Halt the PoW and begin processing transactions.
			fmt.Println("\n---Scheduled Time Reached---")
			naw <- "halt"
			response := <-naw
			if (response != "halted") {
				// Some error
				os.Exit(1)
			}
		}

        LastPoWMax, _ = strconv.ParseUint(<-naw, 10, 64)
        processBlocks(LastPoWMax, count)
    }
}

func setupAccounts() {
    // GET THE NUMBER OF ACCOUNTS FOR THE WALLET
    nWalletAccounts := uint64(0)
    Accounts = AccountList()
    for i := uint64(0); i < NAccounts; i++ {
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

func findFunds() (*big.Int, uint64) {
    // FIND FUNDS
    Total = big.NewInt(0)
    var fakeBalances = GetBalances()
    // Initialize the global balances for every account. This is required.
    Balances = make([]*big.Int, NAccounts)
    // Find the account with the most funds.
    max := big.NewInt(0)
    var nMax uint64
    for i := uint64(0); i < NAccounts; i++ {
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

func distributeFunds(max *big.Int, nMax uint64) {
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
    for i := uint64(0); i < NAccounts; i++ {
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
                ETA = time.Duration((uint64(total) / count) * uint64(NAccounts - uint64(k)))
             }
        }
        fmt.Println()
    }
    fmt.Println("---Finished Setting Up Accounts---")
}

func precomputeBlocks(naw chan string, nMax uint64, iteration int64, maximum uint64, nextTest time.Time) {
    // ITERATE OVER EACH ACCOUNT
    // CREATE BLOCKS
    var ETA time.Duration
    var total time.Duration = 1
    var estimate uint64
	if (iteration % 2 == 0) {
		fmt.Println("---Begin Precomputing PoW (Send Blocks)---")
	} else {
		fmt.Println("---Begin Precomputing PoW (Receive Blocks)---")
	}
    amount := big.NewInt(1)
    // Continue to produce blocks until the scheduled attack time.
    // Estimate how many blocks that will be.
    for i := uint64(0);; i++ {
        // Accounts[i % NAccounts] = the account to process at the moment.
        k := i % NAccounts
        // iter is the 'round' for each account.
        iter := i / NAccounts

		//fmt.Println(len(Blks[k]))
        if i >= uint64(len(Blks[k]) - 1) * NAccounts {
            fmt.Println("Reallocating slices from: ", len(Blks[k]), " to: ", (len(Blks[k]) + 1) * 2)
            // Increase the length of Blks and Hashes.
            for j := uint64(0); j < NAccounts; j++ {
                // Create the new space.
                b := make([]string, (len(Blks[j]) + 1) * 2)
                h := make([]string, (len(Hashes[j]) + 1) * 2)
                // Copy the data into the bigger slices.
                copy(b, Blks[j])
                copy(h, Hashes[j])
                // Reassign the slices to the bigger slices.
                Blks[j] = b
                Hashes[j] = h
            }
        }

        fmt.Print("\rBlock: ", i, "/", estimate, ", ", math.Floor((float64(i) / float64(estimate) * 1000)) / 10, "%")
        fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")
        start := time.Now()
        if iteration % 2 == 0 {
			// Reserve Hashes[k][iter] for the receive blocks.
            Hashes[k][iter + 1], Blks[k][iter + 1] = CreateSendBlock(Accounts[k], Accounts[(i + 1) % NAccounts], Balances[k].String(), amount.String(), Hashes[k][iter])
            Balances[k].Sub(Balances[k], amount)
        } else {
            if uint64(i) > maximum {
				fmt.Println()
                fmt.Println("---Reached the maximum amount of blocks to receive---")
				naw <- "finished"
                naw <- strconv.FormatUint(i, 10)
                return
            }
            // The number should be so large that it will resolve to empty string.
            // The transition between creating send blocks and creating receive blocks
            // requires a lookup to know exactly what the most recent hash was for each account.
            // This is because the interupted process before this will probably be incomplete.
			/*
            var recentHash uint64 = uint64(len(Hashes[k])) - 1
            if iter > 0 {
                recentHash = iter - 1
            }
			if (k == 0) {
				if (iter > 0) {
						Hashes[k][recentHash], Blks[k][recentHash] = CreateReceiveBlock(Accounts[k], Hashes[NAccounts - 1][iter], Hashes[k][iter])
						Balances[k].Add(Balances[k], amount)
				} else {
						// There is nothing to receive because no account has sent anything.
				}
			} else {
			*/
            Hashes[k][iter], Blks[k][iter] = CreateReceiveBlock(Accounts[k], Hashes[(i - 1) % NAccounts][iter + 1], Hashes[k][iter])
            Balances[k].Add(Balances[k], amount)
			//}
        }
        stop := time.Now()
        elapsed := stop.Sub(start)
        total += elapsed
        estimate = uint64(i) + uint64((time.Until(nextTest) / time.Duration(uint64(total) / uint64(i + 1))))
        ETA = time.Duration((uint64(total) / uint64(i + 1)) * (estimate - uint64(i + 1)))

        select {
        case msg := <-naw:
            if (msg == "halt") {
                naw <- "halted"
                fmt.Println("---Halting Precomputation---")
                naw <- strconv.FormatUint(i, 10)
                return
            }
        default:
        }
    }
    fmt.Println()
    fmt.Println("---Finished Precomputing Blocks---")
}

func processBlocks(max uint64, iteration int64) {
    // PROCESS BLOCKS
    var ETA time.Duration
    var total time.Duration = 1
    fmt.Println("---Begin Stress Test (Publishing Blocks)---")
    for i := uint64(0); i < max; i++ {
	    // Accounts[i % NAccounts] = the account to process at the moment.
        k := i % NAccounts
        // iter is the 'round' for each account.
        iter := i / NAccounts

        fmt.Print("\rBlock: ", i, "/", max, ", ", math.Floor((float64(i) / float64(max) * 1000)) / 10, "%")
        fmt.Print(" ETA: ", ETA.String(), " Finish: ", ((time.Now()).Add(ETA)).Format(time.UnixDate), "   \r")
        start := time.Now()
		if (iteration % 2 == 0) {
			Hashes[k][iter] = ProcessBlock(Blks[k][iter + 1])
		} else {
			if (k == 0 && iter == 0) {
				// Nothing to process.
			} else {
				var recentHash uint64 = uint64(len(Hashes[k])) - 1
				if iter > 0 {
					recentHash = iter - 1
				}
				Hashes[k][iter] = ProcessBlock(Blks[k][recentHash])
			}
		}
        stop := time.Now()
        elapsed := stop.Sub(start)
        total += elapsed
        ETA = time.Duration((uint64(total) / (i + 1)) * (max - (i + 1)))
    }
    fmt.Println()
	fmt.Println("\n---Finished Processing Blocks---")
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
