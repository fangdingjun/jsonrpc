package jsonrpc

import (
	"testing"

	log "github.com/fangdingjun/go-log/v5"
)

func TestCall(t *testing.T) {
	log.Default.Level = log.DEBUG

	url := "http://192.168.56.101:8545/"

	c, _ := NewClient(url)

	var ret string

	err := c.Call("eth_getBalance", []string{"0x00CB25f6fD16a52e24eDd2c8fd62071dc29A035c", "latest"}, &ret)
	if err != nil {
		t.Error(err)
		return
	}
	log.Printf("result: %+v", ret)

	c1, err := NewClient("ws://192.168.56.101:8546")
	if err != nil {
		t.Error(err)
		return
	}

	var gas string
	err = c1.Call("eth_gasPrice", []string{}, &gas)
	if err != nil {
		t.Error(err)
		return
	}

	log.Println("gas", gas)

	var r string
	ch, errch, err := c1.Subscribe("eth_subscribe", "eth_subscription", []interface{}{"newHeads"}, &r)

	log.Println("subid", r)
	select {
	case d := <-ch:
		log.Printf("%s", d)
	case e := <-errch:
		log.Println(e)
	}
}
