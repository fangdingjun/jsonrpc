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

	log "github.com/fangdingjun/go-log"
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

// Call call a remote method
func (h *HTTPTransport) Call(method string, args interface{}, reply interface{}) error {
	if args == nil {
		args = []string{}
	}

	r := &request{
		Version: "2.0",
		Method:  method,
		Params:  args,
		ID:      fmt.Sprintf("%d", h.nextID()),
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

// Subscribe subscribe for change
func (h *HTTPTransport) Subscribe(method string, notifyMethod string,
	args interface{}, reply interface{}) (chan json.RawMessage, chan *Error, error) {
	return nil, nil, errors.New("not supported")
}
