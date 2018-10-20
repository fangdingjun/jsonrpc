package jsonrpc

import (
	"log"
	"testing"
)

func TestCall(t *testing.T) {
	url := "http://192.168.56.101:8542/"
	c, _ := NewClient(url)
	c.Debug = true
	var ret interface{}
	err := c.Call("eth_getBalance", []string{"0x00CB25f6fD16a52e24eDd2c8fd62071dc29A035c", "latest"}, &ret)
	if err != nil {
		t.Error(err)
		return
	}
	log.Printf("result: %+v", ret)

	url = "http://admin2:123@192.168.56.101:19011/"

	c, _ = NewClient(url)
	c.Debug = true

	err = c.Call("getbalance", []string{}, &ret)
	if err != nil {
		t.Error(err)
		return
	}

	log.Printf("result: %+v", ret)

	if err = c.Call("fuck", []string{}, &ret); err == nil {
		t.Errorf("expected error, got nil")
		return
	}
	log.Println("got", err)

	if err = c.Call("listreceivedbyaddress", []interface{}{0, false}, &ret); err != nil {
		t.Error(err)
		return
	}
	log.Printf("result: %+v", ret)
}
