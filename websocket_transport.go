package jsonrpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	log "github.com/fangdingjun/go-log"
	"github.com/gorilla/websocket"
)

// WebsocketTransport json rpc over websocket
type WebsocketTransport struct {
	Conn     *websocket.Conn
	URL      string
	Mu       *sync.Mutex
	inflight map[string]*inflightReq
	nextid   uint64
	err      error
}

type inflightReq struct {
	id    string
	ch    chan json.RawMessage
	errch chan *Error
}

var _ Transport = &WebsocketTransport{}

// NewWebsocketTransport create a new websocket transport
func NewWebsocketTransport(uri string) (Transport, error) {
	dialer := &websocket.Dialer{}

	conn, res, err := dialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("http error %d", res.StatusCode)
	}

	w := &WebsocketTransport{
		Conn:     conn,
		URL:      uri,
		inflight: make(map[string]*inflightReq),
		Mu:       new(sync.Mutex),
	}
	go w.readloop()
	return w, nil
}

func (h *WebsocketTransport) readloop() {
	for {
		var res response

		_, data, err := h.Conn.ReadMessage()
		if err != nil {
			h.err = err
			return
		}

		log.Debugf("received: %s", data)

		if err := json.Unmarshal(data, &res); err != nil {
			h.err = err
			return
		}

		if res.ID != "" {
			h.Mu.Lock()
			req, ok := h.inflight[res.ID]
			h.Mu.Unlock()

			if !ok {
				continue
			}
			if res.Error != nil {
				req.errch <- res.Error

				h.Mu.Lock()
				delete(h.inflight, res.ID)
				h.Mu.Unlock()

				continue
			}
			req.ch <- res.Result

			h.Mu.Lock()
			delete(h.inflight, res.ID)
			h.Mu.Unlock()

			continue
		}
		if res.Method != "" {
			h.Mu.Lock()
			req, ok := h.inflight[res.Method]
			h.Mu.Unlock()

			if !ok {
				continue
			}

			if res.Error != nil {
				req.errch <- res.Error
				continue
			}
			req.ch <- res.Params
		}
	}
}

func (h *WebsocketTransport) nextID() uint64 {
	h.Mu.Lock()
	defer h.Mu.Unlock()

	h.nextid++

	return h.nextid
}

// Call call a remote method
func (h *WebsocketTransport) Call(method string, args interface{}, reply interface{}) error {
	if h.err != nil {
		return h.err
	}

	id := fmt.Sprintf("%d", h.nextID())
	req := request{
		ID:      id,
		Version: "2.0",
		Method:  method,
		Params:  args,
	}

	d, err := json.Marshal(req)
	if err != nil {
		return err
	}

	log.Debugf("write %s", d)

	if err := h.Conn.WriteMessage(websocket.TextMessage, d); err != nil {
		return err
	}

	ch := make(chan json.RawMessage, 1)
	errch := make(chan *Error, 1)

	h.Mu.Lock()
	h.inflight[id] = &inflightReq{
		id:    id,
		ch:    ch,
		errch: errch,
	}
	h.Mu.Unlock()

	select {
	case data := <-ch:
		return json.Unmarshal(data, reply)
	case err := <-errch:
		return err
	}
	//return nil
}

// Subscribe subscribe for change
func (h *WebsocketTransport) Subscribe(method string, notifyMethod string,
	args interface{}, reply interface{}) (chan json.RawMessage, chan *Error, error) {

	err := h.Call(method, args, reply)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan json.RawMessage)
	errch := make(chan *Error)

	h.Mu.Lock()
	h.inflight[notifyMethod] = &inflightReq{
		ch:    ch,
		errch: errch,
		id:    notifyMethod,
	}
	h.Mu.Unlock()

	return ch, errch, nil
}
