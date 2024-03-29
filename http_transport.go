package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	log "github.com/fangdingjun/go-log/v5"
)

// HTTPTransport json rpc over http
type HTTPTransport struct {
	Client *http.Client
	URL    string
	nextid uint64
	Mu     *sync.Mutex
}

var _ Transport = &HTTPTransport{}

// NewHTTPTransport create a new http transport
func NewHTTPTransport(uri string, client *http.Client) (Transport, error) {
	_, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	c := client
	if client == nil {
		c = http.DefaultClient
	}
	return &HTTPTransport{
		Client: c,
		URL:    uri,
		Mu:     new(sync.Mutex),
	}, nil
}

func (h *HTTPTransport) nextID() uint64 {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	h.nextid++
	return h.nextid
}

func (h *HTTPTransport) Call(method string, args interface{}, reply interface{}) error {
	return h.CallWithHeader(method, args, reply, nil)
}

// Call call a remote method
func (h *HTTPTransport) CallWithHeader(method string, args interface{}, reply interface{}, header http.Header) error {
	if args == nil {
		args = []string{}
	}

	r := &request{
		Version: "2.0",
		Method:  method,
		Params:  args,
		ID:      h.nextID(),
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	log.Debugf("send %s", data)

	body := bytes.NewReader(data)

	req, err := http.NewRequest("POST", h.URL, body)
	if err != nil {
		return err
	}

	if header != nil {
		for k, v := range header {
			for _, v1 := range v {
				req.Header.Add(k, v1)
			}
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.Debugf("recevied %s", data)

	var res response

	if err = json.Unmarshal(data, &res); err != nil {
		// non 200 response without valid json response
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %s, %s", resp.Status, data)
		}
		return err
	}

	// non 200 response with valid json response
	if res.ID == r.ID && res.Error != nil {
		return res.Error
	}

	// non 200 response without valid json response
	if res.ID == r.ID && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http error: %s, %s", resp.Status, data)
	}

	return json.Unmarshal(res.Result, &reply)
}

// Subscribe subscribe for change
func (h *HTTPTransport) Subscribe(method string, notifyMethod string,
	args interface{}, reply interface{}) (chan json.RawMessage, chan *Error, error) {
	return nil, nil, errors.New("not supported")
}
