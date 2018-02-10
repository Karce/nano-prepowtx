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
    "net/http"
    "encoding/json"
    "io/ioutil"
    "bytes"
)

type RPCRequest struct {
    Action string `json:"action"`
}

type BlockCount struct {
    Count string `json:"count"`
    Unchecked string `json:"unchecked"`
}

func TestRPC() {
    req := RPCRequest{"block_count"}
    bArr, err := json.Marshal(req)
    if err != nil {
	fmt.Println(err)
	return
    }
    fmt.Println(string(bArr))
    
    buf := bytes.NewBuffer(bArr)
    var client *http.Response
    client, err = http.Post("http://localhost:2021", "text/json", buf)

    if err != nil {
	fmt.Println(err)
	return
    }
    
    var bc BlockCount
    var b []byte
    b, err = ioutil.ReadAll(client.Body)
    if err != nil {
	fmt.Println(err)
	return
    }
    fmt.Println(string(b))
    json.Unmarshal(b, &bc)

    // fmt.Println(bc.Count, bc.Unchecked)
}
