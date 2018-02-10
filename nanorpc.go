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

type rpcRequest struct {
    action string
}

type block_count struct {
    count string
    unchecked string
}

func TestRPC() {
    req := rpcRequest{"block_count"}
    bArr, _ := json.Marshal(req)
    buf := bytes.NewBuffer(bArr)
    client, _ := http.Post("localhost:2021", "text/json", buf)
    
    var bc block_count
    b, _ := ioutil.ReadAll(client.Body)
    json.Unmarshal(b, &bc)

    fmt.Println(bc.count)
}
