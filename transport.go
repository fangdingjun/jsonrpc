package jsonrpc

import "encoding/json"

// Transport json rpc transport
type Transport interface {
	Call(method string, args interface{}, reply interface{}) error
	Subscribe(method string, notifyMethod string,
		args interface{}, reply interface{}) (chan json.RawMessage, chan *Error, error)
}
