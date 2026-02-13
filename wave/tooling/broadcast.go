package tooling

import "github.com/gorilla/websocket"

type changeType string

const (
	changeTypeNormalCSS   changeType = "normal"
	changeTypeCriticalCSS changeType = "critical"
	changeTypeOther       changeType = "other"
	changeTypeRebuilding  changeType = "rebuilding"
	changeTypeRevalidate  changeType = "revalidate"
)

type refreshPayload struct {
	ChangeType   changeType `json:"changeType"`
	CriticalCSS  string     `json:"criticalCSS"` // base64
	NormalCSSURL string     `json:"normalCSSURL"`
}

type clientManager struct {
	clients    map[*client]bool
	register   chan *client
	unregister chan *client
	broadcast  chan refreshPayload
	done       chan struct{} // signals manager has fully stopped
}

type client struct {
	id     string
	conn   *websocket.Conn
	notify chan refreshPayload
}

// reloadOpts configures broadcastReload behavior
type reloadOpts struct {
	payload   refreshPayload
	waitApp   bool
	waitVite  bool
	cycleVite bool
}
