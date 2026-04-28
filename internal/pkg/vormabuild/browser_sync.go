package vormabuild

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/netutil"
)

type refresh_payload struct {
	ChangeType  change_type
	CriticalCSS string
	MainCSSURL  string
	BuildError  string
}

type change_type string

const (
	show_rebuilding_overlay change_type = "show_rebuilding_overlay"
	hide_rebuilding_overlay change_type = "hide_rebuilding_overlay"
	show_build_error        change_type = "show_build_error"
	hard_reload             change_type = "hard_reload"
	update_critical_css     change_type = "update_critical_css"
	update_main_css         change_type = "update_main_css"
	client_revalidate       change_type = "revalidate_client"
)

type ws_client struct {
	conn   *websocket.Conn
	notify chan refresh_payload
}

type client_manager struct {
	mu      sync.Mutex
	clients map[*ws_client]bool
}

func new_client_manager() *client_manager {
	return &client_manager{
		clients: make(map[*ws_client]bool),
	}
}

func (cm *client_manager) add(c *ws_client) {
	cm.mu.Lock()
	cm.clients[c] = true
	cm.mu.Unlock()
}

func (cm *client_manager) remove(c *ws_client) {
	cm.mu.Lock()
	if _, ok := cm.clients[c]; ok {
		delete(cm.clients, c)
		close(c.notify)
	}
	cm.mu.Unlock()
}

func (cm *client_manager) broadcast(msg refresh_payload) {
	cm.mu.Lock()
	for c := range cm.clients {
		select {
		case c.notify <- msg:
		default:
		}
	}
	cm.mu.Unlock()
}

func (rs *run_state) dev_refresh_endpoint() string {
	return "/vorma-dev-refresh-" + rs.dev_refresh_token
}

func (rs *run_state) broadcast_build_error(err error) {
	if err == nil {
		return
	}

	rs.client_manager.broadcast(refresh_payload{
		ChangeType: show_build_error,
		BuildError: err.Error(),
	})
}

func (rs *run_state) dev_refresh_handler() http.HandlerFunc {
	rs.mu.Lock()
	client_mgr := rs.client_manager
	rs.mu.Unlock()

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws_upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		c := &ws_client{
			conn:   conn,
			notify: make(chan refresh_payload, 4),
		}
		client_mgr.add(c)

		rs.go_safely(func() {
			defer client_mgr.remove(c)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
		})

		for msg := range c.notify {
			data, err := jsonutil.Serialize(msg)
			if err != nil {
				break
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				break
			}
		}
		client_mgr.remove(c)
		conn.Close()
	}
}

func allow_dev_ws_origin(r *http.Request) bool {
	if !netutil.IsLocalhost(r.Host) {
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return netutil.IsLocalhost(parsed.Host)
}

var ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     allow_dev_ws_origin,
}
