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
	inflight   map[string]*inflightReq
	nextid     uint64
	err        error
	ctx        context.Context
	cancelFunc context.CancelFunc
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
		Conn:     conn,
		URL:      uri,
		inflight: make(map[string]*inflightReq),
		Mu:       new(sync.Mutex),
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

	select {
	case <-h.ctx.Done():
		return ErrConnClosed
	default:
	}

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
