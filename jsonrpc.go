package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

// Client json rpc client
type Client struct {
	// URL is remote url, ex http://username:password@192.168.1.3:1001/jsonrpc
	URL string
	// http client, default is http.DefaultClient
	HTTPClient *http.Client
	id         uint64
	// Debug set to true, log the send/recevied http data
	Debug bool
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
}

// Error rpc error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

// NewClient create a new jsonrpc client
func NewClient(uri string) (*Client, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https supported")
	}

	if u.Host == "" {
		return nil, errors.New("invalid uri")
	}

	return &Client{URL: uri, HTTPClient: http.DefaultClient}, nil
}

func (c *Client) nextID() uint64 {
	c.id++
	return c.id
}

// Call invoke a method with args and return reply
func (c *Client) Call(method string, args interface{}, reply interface{}) error {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	if args == nil {
		args = []string{}
	}

	r := &request{
		Version: "2.0",
		Method:  method,
		Params:  args,
		ID:      c.nextID(),
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if c.Debug {
		log.Println("send", string(data))
	}

	body := bytes.NewBuffer(data)

	req, err := http.NewRequest("POST", c.URL, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if c.Debug {
		log.Println("recevied", string(data))
	}

	var res response

	if err = json.Unmarshal(data, &res); err != nil {
		// non 200 response without valid json response
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %s", resp.Status)
		}
		return err
	}

	// non 200 response with valid json response
	if res.ID == r.ID && res.Error != nil {
		return res.Error
	}

	// non 200 response without valid json response
	if res.ID == r.ID && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http error: %s", resp.Status)
	}

	return json.Unmarshal(res.Result, &reply)
}
