jsonrpc
=======

simple jsonrpc 2.0 client


usage
====
example

    package main

    import (
        "github.com/fangdingjun/jsonrpc"
        "log"
    )

    type result struct{
        R1 string `json:"r1"`
        R2 string `json:"r2"`
    }

    func main(){
        client, _ := jsonrpc.NewClient("http://admin:123@127.0.0.1:2312/jsonrpc")
        // client.Debug = true
        // client.HTTPClient = &http.Client{...}

        var ret result
        var args = []interface{}{1, "a", 2}
        err := client.Call("some_method", args, &ret)
        if err != nil{
            log.Fatal(err)
        }
        log.Println(ret)
    }
