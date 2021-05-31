package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// Client json rpc client
type Client struct {
	// URL is remote url,
	// example
	//     http://username:password@192.168.1.3:1001/jsonrpc
	//     ws://192.168.0.1:9121/
	URL       string
	Transport Transport
}

type request struct {
	Version string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      uint64      `json:"id"`
}

type response struct {
	Version string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *Error          `json:"error"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// Error rpc error
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("code: %d, message: %s, data: %s",
		e.Code, e.Message, e.Data)
}

// NewClient create a new jsonrpc client
func NewClient(uri string) (*Client, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	t := ""
	switch u.Scheme {
	case "http", "https":
		t = "http"
	case "ws", "wss":
		t = "ws"
	default:
		return nil, fmt.Errorf("not supported %s", u.Scheme)
	}

	if t == "http" {
		tr, _ := NewHTTPTransport(uri, nil)
		return &Client{Transport: tr, URL: uri}, nil
	}
	if t == "ws" {
		tr, _ := NewWebsocketTransport(context.Background(), uri)
		return &Client{Transport: tr, URL: uri}, nil
	}
	return nil, errors.New("not supported")
}

// Call invoke a method with args and return reply
func (c *Client) Call(method string, args interface{}, reply interface{}) error {
	return c.Transport.Call(method, args, reply)
}

// Subscribe subscribe for change
func (c *Client) Subscribe(method, notifyMethod string,
	args interface{}, reply interface{}) (chan json.RawMessage, chan *Error, error) {

	return c.Transport.Subscribe(method, notifyMethod, args, reply)
}
