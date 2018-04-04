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
)

var Wallet string = ""

type RPCRequest struct {
    Action string `json:"action"`
}

type BlockCount struct {
    Count string `json:"count"`
    Unchecked string `json:"unchecked"`
}

func Init(wallet string) {
    Wallet = wallet
}

func MakeRequest(data interface{}) ([]byte) {
    // req := RPCRequest{"block_count"}
    bArr, err := json.Marshal(data)
    if err != nil {
        fmt.Println(err)
        os.Exit(1) 
    }
    fmt.Println(string(bArr))
    
    buf := bytes.NewBuffer(bArr)
    var client *http.Response
    client, err = http.Post("http://localhost:2021", "text/json", buf)

    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    
    // var bc BlockCount
    var b []byte
    b, err = ioutil.ReadAll(client.Body)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    fmt.Println(string(b))
    // json.Unmarshal(b, &bc)
    // fmt.Println(bc.Count, bc.Unchecked)
    return b
}

type BCRequest struct {
    Action string `json:"action"`
    Type string `json:"type"`
    Wallet string `json:"wallet"`
    Account string `json:"wallet"`
    Destination string `json:"destination"`
    Balance string `json:"balance"`
    Amount string `json:"amount"`
    Previous string `json:"previous"`
}

type BCResponse struct {
    Hash string `json:"hash"`
    Block string `json:"block"`
}

type Block struct {
    Type string `json:"type"`
    Previous string `json:"previous"`
    Destination string `json:"destination"`
    Balance string `json:"balance"`
    Work string `json:"work"`
    Signature string `json:"signature"`
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
func CreateSendBlock() (string) {
    var req BCRequest
    req.Action = "block_create"
    req.Type = "send"
    req.Wallet = Wallet
    // req.Account = 
    // req.Destination = 
    // req.Balance = 
    // req.Amount = 
    // req.Previous = 
    b := MakeRequest(req)

    var bcr BCResponse
    json.Unmarshal(b, &bcr)
    // Unmarshal the block string here too.
    var block Block
    json.Unmarshal([]byte(bcr.Block), &block)

    fmt.Println(bcr.Hash)
    fmt.Println(block.Work)
    return bcr.Hash
}

func ProcessBlock() {

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
