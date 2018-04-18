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
    "os"
    "fmt"
    "net/http"
    "encoding/json"
    "io/ioutil"
    "bytes"
    "time"
)

type RPCRequest struct {
    Action string `json:"action"`
}

type BlockCount struct {
    Count string `json:"count"`
    Unchecked string `json:"unchecked"`
}

func MakeRequest(data interface{}) ([]byte) {
    // req := RPCRequest{"block_count"}
    bArr, err := json.Marshal(data)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    // fmt.Println(string(bArr))
    buf := bytes.NewBuffer(bArr)
    var client *http.Response
    client, err = http.Post("http://localhost:7076", "text/json", buf)

    for err != nil {
        fmt.Println(err)
        fmt.Println("Trying again in 10 seconds")
        time.Sleep(time.Duration(10) * time.Second)
        client, err = http.Post("http://localhost:7076", "text/json", buf)
    }

    // var bc BlockCount
    var b []byte
    b, err = ioutil.ReadAll(client.Body)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    // fmt.Println(string(b))
    // json.Unmarshal(b, &bc)
    // fmt.Println(bc.Count, bc.Unchecked)
    return b
}

type BCRequest struct {
    Action string `json:"action"`
    Type string `json:"type"`
    Wallet string `json:"wallet"`
    Account string `json:"account"`
    Destination string `json:"destination"`
    Balance string `json:"balance"`
    Amount string `json:"amount"`
    Previous string `json:"previous"`
}

type BCResponse struct {
    Hash string `json:"hash"`
    Block string `json:"block"`
}

type BRRequest struct {
    Action string `json:"action"`
    Type string `json:"type"`
    Wallet string `json:"wallet"`
    Account string `json:"account"`
    Source string `json:"source"`
    Previous string `json:"previous"`
}

type Block struct {
    Type string `json:"type"`
    Previous string `json:"previous"`
    Destination string `json:"destination"`
    Balance string `json:"balance"`
    Work string `json:"work"`
    Signature string `json:"signature"`
}

type ABRequest struct {
    Action string `json:"action"`
    Account string `json:"account"`
}

type AHRequest struct {
    Action string `json:"action"`
    Account string `json:"account"`
    Count string `json:"count"`
}

type AHResponse struct {
    History []AHHistory `json:"history"`
}

type AHHistory struct {
    Hash string `json:"hash"`
    Type string `json:"type"`
    Account string `json:"account"`
    Amount string `json:"amount"`
}

// Use wallet/generate wallet.
// Search for xrb on wallet accounts using wallet_balances.
// Generate accounts until have enough (100) with accounts_create. Search by accounts_list.
// Send enough from the first account to the other accounts. On Block Lattice. Build send and process, republish if didn't work. 30000 / 100 = raw per account, 300 transactions per block.
// Retrieve frontier block on accounts with account_info.
// Generate a single account to be used as the destination account.
// Begin generating n blocks per account. Store them.
// Process n blocks, publishing them to the network.
// Iterate through accounts, process one account/block at a time so the network
// doesn't reject them.
func CreateSendBlock(account string, dest string, balance string, amount string, previous string) (string, string) {
    var req BCRequest
    req.Action = "block_create"
    req.Type = "send"
    req.Wallet = Wallet
    req.Account = account
    req.Destination = dest
    // Get balance
    if (balance == "") {
        var abr ABRequest
        abr.Action = "account_balance"
        abr.Account = account
        a := MakeRequest(abr)
        var wab WABalance
        json.Unmarshal(a, &wab)
        req.Balance = wab.Balance // The current balance of the account before this transaction. Can retrieve this and keep track of it.
    } else {
        req.Balance = balance
    }
    req.Amount = amount
    // Find the last block hashes with Account_List.
    // From that point keep track of the block hashes.

    if (previous == "") {
        req.Previous = GetPreviousBlock(account)
    } else {
        req.Previous = previous
    }
    b := MakeRequest(req)

    var bcr BCResponse
    json.Unmarshal(b, &bcr)
    // Unmarshal the block string here too.
    var block Block
    json.Unmarshal([]byte(bcr.Block), &block)
    return bcr.Hash, bcr.Block
}

func CreateReceiveBlock(account string, source string, previous string) (string, string) {
    var req BRRequest
    req.Action = "block_create"
    req.Type = "receive"
    req.Wallet = Wallet
    req.Account = account
    req.Source = source

    // Find the last block hashes with Account_List.
    // From that point keep track of the block hashes.

    if (previous == "") {
        req.Previous = GetPreviousBlock(account)
    } else {
        req.Previous = previous
    }
    b := MakeRequest(req)

    var bcr BCResponse
    json.Unmarshal(b, &bcr)
    // Unmarshal the block string here too.
    var block Block
    json.Unmarshal([]byte(bcr.Block), &block)
    return bcr.Hash, bcr.Block
}

func GetPreviousBlock(account string) (string) {
    var ahr AHRequest
    ahr.Action = "account_history"
    ahr.Account = account
    ahr.Count = "1"
    c := MakeRequest(ahr)
    var ahre AHResponse
    json.Unmarshal(c, &ahre)
    if (len(ahre.History) >= 1) {
        return ahre.History[0].Hash // Previous block hash. Keep track of the last block for the account. 
    } else {
        return ""
    }
}

func GetPendingBlocks(account, count string) ([]string) {
    var preq PRequest
    preq.Action = "pending"
    preq.Account = account
    preq.Count = count

    a := MakeRequest(preq)
    var pr PResponse
    json.Unmarshal(a, &pr)
    return pr.Blocks
}

type PRequest struct {
    Action string `json:"action"`
    Account string `json:"account"`
    Count string `json:"count"`
}

type PResponse struct {
    Blocks []string `json:"blocks"`
}

type SRequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
    Source string `json:"source"`
    Destination string `json:"destination"`
    Amount string `json:"amount"`
}

type SResponse struct {
    Block string `json:"block"`
}

func Send(source, destination, amount string) (string) {
    var sreq SRequest
    sreq.Action = "send"
    sreq.Wallet = Wallet
    sreq.Source = source
    sreq.Destination = destination
    sreq.Amount = amount

    a := MakeRequest(sreq)

    var sres SResponse
    json.Unmarshal(a, &sres)

    return sres.Block

}

type RRequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
    Account string `json:"account"`
    Block string `json:"block"`
}

type RResponse struct {
    Hash string `json:"hash"`
}

func ReceiveBlock(account string, block string) (string) {
    var rreq RRequest
    rreq.Action = "receive"
    rreq.Wallet = Wallet
    rreq.Account = account
    rreq.Block = block

    a := MakeRequest(rreq)

    var rres RResponse
    json.Unmarshal(a, &rres)

    return rres.Hash
}

type PBRequest struct {
    Action string `json:"action"`
    Block string `json:"block"`
}

type PBResponse struct {
    Hash string `json:"hash"`
}

func ProcessBlock(blk string) (string) {
    var pbr PBRequest
    pbr.Action = "process"
    pbr.Block = blk
    a := MakeRequest(pbr)

    var pbh PBResponse
    json.Unmarshal(a, &pbh)

    return pbh.Hash
}

type ACRequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
}

type ACRespone struct {
    Account string `json:account"`
}

// Logic to control how many accounts to generate. If we need 100 accounts,
// make the requests to GenerateAccounts.
// Control the accounts that we get, keep track of them and their balance.
func GenerateAccounts() {
}

// Make a request to generate a single account with the wallet.
func GenerateAccount() (string) {
   var req ACRequest
   req.Action = "account_create"
   req.Wallet = Wallet

   a := MakeRequest(req)

   var acr ACRespone
   json.Unmarshal(a, &acr)
   return acr.Account
}

type ALRequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
}

type ALResponse struct {
    Accounts []string `json:"accounts"`
}

func AccountList() ([]string) {
    var req ALRequest
    req.Action = "account_list"
    req.Wallet = Wallet

    a := MakeRequest(req)

    var alr ALResponse
    json.Unmarshal(a, &alr)
    return alr.Accounts
}

type WCRequest struct {
    Action string `json:"action"`
}

type WCResponse struct {
    Wallet string `json:"wallet"`
}

func GenerateWallet() (string) {
    var req WCRequest
    req.Action = "wallet_create"

    a := MakeRequest(req)

    var wcr WCResponse
    json.Unmarshal(a, &wcr)
    return wcr.Wallet
}

type WARequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
}

type WAResponse struct {
    Balances map[string]WABalance `json:"balances"`
}

type WABalance struct {
    Balance string `json:"balance"`
    Pending string `json:"pending"`
}

func GetBalances() (map[string]WABalance) {
    var req WARequest
    req.Action = "wallet_balances"
    req.Wallet = Wallet

    a := MakeRequest(req)

    var war WAResponse
    json.Unmarshal(a, &war)
    return war.Balances
}
