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
    "os"
    "fmt"
    "net/http"
    "encoding/json"
    "io/ioutil"
    "bytes"
    "time"
)

/*
 * This file is split up between json structs and functions that supply those structs.
 * MakeRequest and Unmarshal deal with handling generic requests and responses.
 * The other functions are designed specifically to handle their specific request
 * but most of them follow the same basic structure.
 */

// Any request that is only a single action.
type RPCRequest struct {
    Action string `json:"action"`
}

// MakeRequest handles any json struct and sends those requests over
// HTTP POST to the nano-node server.
// It then reads the response body and returns it as a byte array.
func MakeRequest(data interface{}) ([]byte) {
    bArr, err := json.Marshal(data)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    //fmt.Println(string(bArr))

    buf := bytes.NewBuffer(bArr)
    var client *http.Response
    client, err = http.Post("http://localhost:7076", "text/json", buf)

    for err != nil {
        fmt.Println(err)
        fmt.Println("Trying again in 10 seconds")
        time.Sleep(time.Duration(10) * time.Second)
        client, err = http.Post("http://localhost:7076", "text/json", buf)
    }

    var b []byte
    b, err = ioutil.ReadAll(client.Body)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    //fmt.Println(string(b))

    return b
}

type EResponse struct {
	Error string `json:"error"`
}

// Wrapper for json.Unmarshal that handles errors.
func Unmarshal(data []byte, v interface{}) {
	var eres EResponse
	json.Unmarshal(data, &eres)
	if (eres.Error != "") {
		fmt.Println("Error:", eres.Error)
		os.Exit(1)
	}

    err := json.Unmarshal(data, v)
    if (err != nil) {
        fmt.Println(err)
        os.Exit(1)
    }
}

// Block count request.
type BlockCount struct {
    Count string `json:"count"`
    Unchecked string `json:"unchecked"`
}

// Create send block request.
type BSRequest struct {
    Action string `json:"action"`
    Type string `json:"type"`
    Wallet string `json:"wallet"`
    Account string `json:"account"`
    Destination string `json:"destination"`
    Balance string `json:"balance"`
    Amount string `json:"amount"`
    Previous string `json:"previous"`
}

// Create receive block request.
type BRRequest struct {
    Action string `json:"action"`
    Type string `json:"type"`
    Wallet string `json:"wallet"`
    Account string `json:"account"`
    Source string `json:"source"`
    Previous string `json:"previous"`
}

// Create block response.
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

// Account balance request.
type ABRequest struct {
    Action string `json:"action"`
    Account string `json:"account"`
}

func CreateSendBlock(account string, dest string, balance string, amount string, previous string) (string, string) {
    bsreq := BSRequest{"block_create", "send", Wallet, account, dest, balance, amount, previous}

    // Get the balance if it is unknown.
    if (balance == "") {
        abreq := ABRequest{"account_balance", account}

        a := MakeRequest(abreq)

        var wab WABalance
        Unmarshal(a, &wab)
        // The current balance of the account before this transaction.
        bsreq.Balance = wab.Balance
    }

    // Find the last block hashes with Account_List if it is unknown.
    // From that point keep track of the block hashes.

    if (previous == "") {
        bsreq.Previous = GetPreviousBlock(account)
    }

    a := MakeRequest(bsreq)

    var bcres BCResponse
    Unmarshal(a, &bcres)

    return bcres.Hash, bcres.Block
}

func CreateReceiveBlock(account string, source string, previous string) (string, string) {
    brreq := BRRequest{"block_create", "receive", Wallet, account, source, previous}

    // Find the last block hashes with Account_List if it is unknown.
    // From that point keep track of the block hashes.

    if (previous == "") {
        brreq.Previous = GetPreviousBlock(account)
    }

    a := MakeRequest(brreq)

    var bcres BCResponse
    Unmarshal(a, &bcres)

    return bcres.Hash, bcres.Block
}

// Process block request and response.
type PBRequest struct {
    Action string `json:"action"`
    Block string `json:"block"`
}

type PBResponse struct {
    Hash string `json:"hash"`
}

func ProcessBlock(blk string) (string) {
    pbreq := PBRequest{"process", blk}

    a := MakeRequest(pbreq)

    var pbres PBResponse
    Unmarshal(a, &pbres)

    return pbres.Hash
}

// Account history request and response.
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

func GetPreviousBlock(account string) (string) {
    ahreq := AHRequest{"account_history", account, "1"}

    a := MakeRequest(ahreq)

    var ahres AHResponse
    Unmarshal(a, &ahres)

    if (len(ahres.History) >= 1) {
        return ahres.History[0].Hash // Previous block hash. Keep track of the last block for the account.
    } else {
        return ""
    }
}

// Pending block request and response.
type PRequest struct {
    Action string `json:"action"`
    Account string `json:"account"`
    Count string `json:"count"`
}

type PResponse struct {
    Blocks []string `json:"blocks"`
}

func GetPendingBlocks(account, count string) ([]string) {
    preq := PRequest{"pending", account, count}

    a := MakeRequest(preq)

    var pres PResponse
    Unmarshal(a, &pres)

    return pres.Blocks
}

// Send request and response.
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
    sreq := SRequest{"send", Wallet, source, destination, amount}

    a := MakeRequest(sreq)

    var sres SResponse
    Unmarshal(a, &sres)

    return sres.Block
}

// Receive block request and response.
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
    rreq := RRequest{"receive", Wallet, account, block}

    a := MakeRequest(rreq)

    var rres RResponse
    Unmarshal(a, &rres)

    return rres.Hash
}

// Account create request and response.
type ACRequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
}

type ACRespone struct {
    Account string `json:account"`
}

// Make a request to generate a single account with the wallet.
func GenerateAccount() (string) {
   acreq := ACRequest{"account_create", Wallet}

   a := MakeRequest(acreq)

   var acres ACRespone
   Unmarshal(a, &acres)

   return acres.Account
}

func GenerateAccounts() {}

// Account list request and response.
type ALRequest struct {
    Action string `json:"action"`
    Wallet string `json:"wallet"`
}

type ALResponse struct {
    Accounts []string `json:"accounts"`
}

func AccountList() ([]string) {
    alreq := ALRequest{"account_list", Wallet}

    a := MakeRequest(alreq)

    var alres ALResponse
    Unmarshal(a, &alres)

    return alres.Accounts
}

// Wallet create response.
type WCResponse struct {
    Wallet string `json:"wallet"`
}

func GenerateWallet() (string) {
    wcreq := RPCRequest{"wallet_create"}

    a := MakeRequest(wcreq)

    var wcres WCResponse
    Unmarshal(a, &wcres)

    return wcres.Wallet
}

// Wallet account balances request and response.
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
    req := WARequest{"wallet_balances", Wallet}

    a := MakeRequest(req)

    var wares WAResponse
    Unmarshal(a, &wares)

    return wares.Balances
}
