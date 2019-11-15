package sherpa

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchd/snowglobe/models"
	"github.com/gcash/bchutil"
	"github.com/gorilla/websocket"
)

type socketChan chan (*websocket.Conn)

type notif struct {
	VertexHash       string `json:"vertex_hash"`
	FinalizationTime string `json:"finalization_time"`
}

type Server struct {
	httpServer *http.Server

	notifications *boundedSlice

	errCh      chan error
	newNotifCh chan notif
	newConnCh  chan *websocket.Conn
	doneConnCh chan *websocket.Conn

	// Control flow channels
	quitCh chan struct{}
	doneCh chan struct{}
}

func New() (*Server, error) {
	s := &Server{
		notifications: newBoundedSlice(8),

		errCh:      make(chan error),
		newNotifCh: make(chan notif),
		newConnCh:  make(chan *websocket.Conn),
		doneConnCh: make(chan *websocket.Conn),

		quitCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// Start listening for events
	go s.listenChans()

	// Connect to bchd and start receiving events
	err := connectBchdWebsocket(s.newNotifCh)
	if err != nil {
		return nil, err
	}

	// Create an HTTP server for the API
	s.httpServer, err = newHTTPServer(s.newConnCh, s.doneConnCh, s.notifications)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) Stop() error {
	close(s.quitCh)
	<-s.doneCh

	close(s.errCh)
	close(s.newNotifCh)
	close(s.newConnCh)
	close(s.doneConnCh)

	return s.httpServer.Shutdown(context.Background())
}

func (s *Server) Errors() <-chan error {
	return s.errCh
}

func (s *Server) listenChans() {
	conns := make(map[*websocket.Conn]struct{})
	for {
		select {
		case <-s.quitCh:
			close(s.doneCh)
			return

		// Add incoming connections to the set
		case conn := <-s.newConnCh:
			conns[conn] = struct{}{}

		// Remove disconnecting connections from the set
		case conn := <-s.doneConnCh:
			delete(conns, conn)

		// Broadcast new notifications to all clients
		case n := <-s.newNotifCh:
			s.notifications.append(n)
			out, err := json.MarshalIndent(&n, "", "    ")
			if err != nil {
				s.errCh <- err
				continue
			}

			for conn := range conns {
				err = conn.WriteMessage(websocket.TextMessage, out)
				if err != nil {
					s.errCh <- err
					continue
				}
			}
		}
	}
}

func newHTTPServer(newConn socketChan, doneConn socketChan, notifications *boundedSlice) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", newWebsocketHandler(newConn, doneConn))
	mux.HandleFunc("/notifications", newNotificationsHandler(notifications))
	mux.Handle("/", http.StripPrefix("/", http.HandlerFunc(static_handler)))

	httpServer := &http.Server{Addr: ":3000", Handler: mux}

	err := httpServer.ListenAndServe()
	if err != nil {
		return nil, err
	}

	return httpServer, nil
}

func connectBchdWebsocket(notificationChan chan notif) error {
	// Only override the handlers for notifications you care about.
	// Also note most of these handlers will only be called if you register
	// for notifications.  See the documentation of the rpcclient
	// NotificationHandlers type for more details about each handler.
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnAvaFinalization: func(vr *models.VoteRecord) {
			notificationChan <- notif{
				VertexHash:       vr.VertexHash.String(),
				FinalizationTime: vr.FinalizationLatency().String(),
			}
		},
	}

	// Connect to local bchd RPC server using websockets.
	bchdHomeDir := bchutil.AppDataDir("bchd", false)
	certs, err := ioutil.ReadFile(filepath.Join(bchdHomeDir, "rpc.cert"))
	if err != nil {
		return err
	}
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:8334",
		Endpoint:     "ws",
		User:         "alice",
		Pass:         "letmein",
		Certificates: certs,
	}
	client, err := rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		return err
	}
	if err := client.NotifyAvalanche(); err != nil {
		return err
	}
	return nil
}
