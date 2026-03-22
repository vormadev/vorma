package wavebuild

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/wave/internal/constants"
)

/////////////////////////////////////////////////////////////////////
/////// Payloads
/////////////////////////////////////////////////////////////////////

type refresh_payload struct {
	ChangeType        change_type `json:"change_type"`
	CriticalCSS       string      `json:"critical_css,omitempty"`
	NonCriticalCSSURL string      `json:"non_critical_css_url,omitempty"`
	BuildError        string      `json:"build_error,omitempty"`
}

type change_type string

const (
	change_type_show_rebuilding_overlay change_type = "show_rebuilding_overlay"
	change_type_hide_rebuilding_overlay change_type = "hide_rebuilding_overlay"
	change_type_build_error             change_type = "build_error"
	change_type_hard_reload             change_type = "hard_reload"
	change_type_critical_css            change_type = "critical_css"
	change_type_non_critical_css        change_type = "non_critical_css"
	change_type_data_revalidate         change_type = "data_revalidate"
)

/////////////////////////////////////////////////////////////////////
/////// Client management
/////////////////////////////////////////////////////////////////////

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

/////////////////////////////////////////////////////////////////////
/////// Browser sync server
/////////////////////////////////////////////////////////////////////

type browser_sync struct {
	port    int
	token   string
	clients *client_manager
	server  *http.Server
	logger  *slog.Logger
}

func new_browser_sync(
	port int,
	token string,
	logger *slog.Logger,
) *browser_sync {
	return &browser_sync{
		port:    port,
		token:   token,
		clients: new_client_manager(),
		logger:  logger,
	}
}

func (bs *browser_sync) start() {
	mux := http.NewServeMux()
	mux.HandleFunc(
		constants.DEV_REFRESH_EVENTS_PATH_PREFIX+bs.token,
		bs.handle_ws,
	)

	bs.server = &http.Server{
		Addr:    ":" + strconv.Itoa(bs.port),
		Handler: mux,
	}

	go func() {
		bs.logger.Info("starting browser sync server",
			"port", bs.port,
		)
		if err := bs.server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			bs.logger.Error("browser sync server failed", "err", err)
		}
	}()
}

func (bs *browser_sync) stop() {
	if bs.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = bs.server.Shutdown(ctx)
	bs.logger.Info("browser sync server stopped")
}

// send_rebuilding tells all connected browsers to show the
// rebuilding overlay. Called before the build starts.
func (bs *browser_sync) send_rebuilding() {
	bs.logger.Debug("Showing rebuilding overlay")
	bs.clients.broadcast(refresh_payload{
		ChangeType: change_type_show_rebuilding_overlay,
	})
}

func (bs *browser_sync) hide_rebuilding() {
	bs.logger.Debug("Hiding rebuilding overlay")
	bs.clients.broadcast(refresh_payload{
		ChangeType: change_type_hide_rebuilding_overlay,
	})
}

func (bs *browser_sync) show_build_error(build_err string) {
	bs.logger.Debug("Showing build error overlay")
	bs.clients.broadcast(refresh_payload{
		ChangeType: change_type_build_error,
		BuildError: build_err,
	})
}

// settle tells all connected browsers what happened after a build
// cycle completes. The effect set determines the message type.
// Hard reload supersedes everything else.
func (bs *browser_sync) settle(
	fx *set.Set[Effect],
	critical_css []byte,
	non_critical_css_url string,
) {
	if fx.Has(EffectHardReloadBrowser) {
		bs.logger.Info("hard reloading browser")
		bs.clients.broadcast(refresh_payload{
			ChangeType: change_type_hard_reload,
		})
		return
	}

	if fx.Has(EffectRevalidateClientData) {
		bs.logger.Info("triggering client data revalidation")
		bs.clients.broadcast(refresh_payload{
			ChangeType: change_type_data_revalidate,
		})
		// do not early return because revalidate_client_data
		// can be combined with CSS hot refresh
	}

	if fx.HasAny(effect_build_critical_css, effect_build_non_critical_css) {
		bs.logger.Info("hot reloading CSS")
		if fx.Has(effect_build_critical_css) {
			bs.clients.broadcast(refresh_payload{
				ChangeType:  change_type_critical_css,
				CriticalCSS: bytesutil.ToBase64(critical_css),
			})
		}
		if fx.Has(effect_build_non_critical_css) {
			bs.clients.broadcast(refresh_payload{
				ChangeType:        change_type_non_critical_css,
				NonCriticalCSSURL: non_critical_css_url,
			})
		}
		return
	}
}

/////////////////////////////////////////////////////////////////////
/////// WebSocket handler
/////////////////////////////////////////////////////////////////////

var ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// This is safe because the events endpoint path itself contains an
	// unguessable random token unknown to attackers.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (bs *browser_sync) handle_ws(w http.ResponseWriter, r *http.Request) {
	conn, err := ws_upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &ws_client{
		conn:   conn,
		notify: make(chan refresh_payload, 4),
	}
	bs.clients.add(c)

	// read pump — keeps connection alive, detects close
	go func() {
		defer bs.clients.remove(c)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// write pump
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
	conn.Close()
}
