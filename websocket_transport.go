package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	log "github.com/fangdingjun/go-log"
	"github.com/gorilla/websocket"
)

// WebsocketTransport json rpc over websocket
type WebsocketTransport struct {
	Conn       *websocket.Conn
	URL        string
	Mu         *sync.Mutex
	inflight   map[uint64]*inflightReq
	nextid     uint64
	err        error
	ctx        context.Context
	cancelFunc context.CancelFunc
	subscribes map[string][]*inflightReq
}

type inflightReq struct {
	id    string
	ch    chan json.RawMessage
	errch chan *Error
}

// ErrConnClosed error for connection closed
var ErrConnClosed = errors.New("connection closed")

var _ Transport = &WebsocketTransport{}

// NewWebsocketTransport create a new websocket transport
func NewWebsocketTransport(ctx context.Context, uri string) (Transport, error) {
	var dialer = &websocket.Dialer{}
	conn, res, err := dialer.DialContext(ctx, uri, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("http error %d", res.StatusCode)
	}

	w := &WebsocketTransport{
		Conn:       conn,
		URL:        uri,
		inflight:   make(map[uint64]*inflightReq),
		subscribes: make(map[string][]*inflightReq),
		Mu:         new(sync.Mutex),
	}
	w.ctx, w.cancelFunc = context.WithCancel(ctx)
	go w.readloop()
	return w, nil
}

// Context return the context transport used
func (h *WebsocketTransport) Context() context.Context {
	return h.ctx
}

func (h *WebsocketTransport) readloop() {
	defer func() {
		//log.Debugf("close websocket connection")
		h.Conn.Close()
		//log.Debugf("cancel context")
		h.cancelFunc()
	}()

	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}
		var res response

		_, data, err := h.Conn.ReadMessage()
		if err != nil {
			log.Errorln(err)
			h.err = err
			return
		}

		log.Debugf("received: %s", data)

		if err := json.Unmarshal(data, &res); err != nil {
			h.err = err
			return
		}

		if res.ID != 0 {
			// response

			h.Mu.Lock()
			req, ok := h.inflight[res.ID]
			h.Mu.Unlock()

			if !ok {
				log.Warnf("handler for id %s not exists", res.ID)
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
			// notify

			h.Mu.Lock()
			reqs, ok := h.subscribes[res.Method]
			h.Mu.Unlock()

			if !ok {
				log.Warnf("handler for method %s not exists", res.Method)
				continue
			}

			for _, req := range reqs {
				if res.Error != nil {
					req.errch <- res.Error
				} else {
					req.ch <- res.Params
				}
			}
			continue
		}

		log.Warnf("unhandled message %s", data)
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

	select {
	case <-h.ctx.Done():
		return ErrConnClosed
	default:
	}

	if h.err != nil {
		return h.err
	}

	id := h.nextID()
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

	ch := make(chan json.RawMessage, 1)
	errch := make(chan *Error, 1)

	h.Mu.Lock()
	h.inflight[id] = &inflightReq{
		id:    fmt.Sprintf("%d", id),
		ch:    ch,
		errch: errch,
	}
	h.Mu.Unlock()

	log.Debugf("write %s", d)

	if err := h.Conn.WriteMessage(websocket.TextMessage, d); err != nil {
		h.Mu.Lock()
		delete(h.inflight, id)
		h.Mu.Unlock()
		return err
	}

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

	ch := make(chan json.RawMessage)
	errch := make(chan *Error)

	req := &inflightReq{
		ch:    ch,
		errch: errch,
		id:    notifyMethod,
	}

	h.Mu.Lock()
	sub, ok := h.subscribes[notifyMethod]
	if ok {
		h.subscribes[notifyMethod] = append(sub, req)
	} else {
		h.subscribes[notifyMethod] = []*inflightReq{req}
	}
	h.Mu.Unlock()

	err := h.Call(method, args, reply)
	if err != nil {
		h.Mu.Lock()
		sub := h.subscribes[notifyMethod]
		n := len(sub)
		for i := 0; i < n; i++ {
			if sub[i] == req {
				if i == 0 {
					// first
					h.subscribes[notifyMethod] = sub[1:]
					break
				}
				if i == (n - 1) {
					// last
					h.subscribes[notifyMethod] = sub[:n-1]
					break
				}
				h.subscribes[notifyMethod] = append(sub[:i], sub[i+1:]...)
				break
			}
		}
		h.Mu.Unlock()
		return nil, nil, err
	}

	return ch, errch, nil
}
