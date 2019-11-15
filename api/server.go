package api

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

type Server struct {
	httpServer *http.Server
	notifCache *notificationCache

	// Event channels
	errCh      chan error
	newNotifCh chan notif
	newConnCh  socketChan
	doneConnCh socketChan

	// Control flow channels
	quitCh emptyChan
	doneCh emptyChan
}

func New() (*Server, error) {
	s := &Server{
		notifCache: newNotificationCache(8),

		errCh:      make(chan error),
		newNotifCh: make(chan notif),
		newConnCh:  make(socketChan),
		doneConnCh: make(socketChan),

		quitCh: make(emptyChan),
		doneCh: make(emptyChan),
	}

	// Start listening for events
	go s.listenChans()

	// Connect to bchd and start receiving events
	err := connectBchdWebsocket(newAvaCallback(s.newNotifCh))
	if err != nil {
		return nil, err
	}

	// Create an HTTP server for the API
	s.httpServer, err = newHTTPServer(s.newConnCh, s.doneConnCh, s.notifCache)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) Port() int {
	return 3000
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
			s.notifCache.append(n)
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

func connectBchdWebsocket(cb avaCallbackFn) error {
	// Load certs file from default location
	certs, err := ioutil.ReadFile(filepath.Join(bchutil.AppDataDir("bchd", false), "rpc.cert"))
	if err != nil {
		return err
	}

	// Create a client configured to call our callback in response to new
	// Avalanche events
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         "localhost:8334",
		Endpoint:     "ws",
		User:         "alice",
		Pass:         "letmein",
		Certificates: certs,
	}, &rpcclient.NotificationHandlers{OnAvaFinalization: cb})
	if err != nil {
		return err
	}

	// Start listening for Avalanche notifications
	return client.NotifyAvalanche()
}

func newAvaCallback(newNotifCh chan notif) avaCallbackFn {
	return func(vr *models.VoteRecord) {
		newNotifCh <- notif{
			VertexHash:       vr.VertexHash.String(),
			VertexType:       vr.VertexType,
			FinalizationTime: vr.FinalizationLatency().String()}
	}
}

func newHTTPServer(newConn socketChan, doneConn socketChan, notifCache *notificationCache) (*http.Server, error) {
	mux := http.NewServeMux()

	// Server UI
	// TODO: Seperate out this static UI into a separate deployable chunk
	mux.Handle("/", http.StripPrefix("/", http.HandlerFunc(static_handler)))

	// Add API routes
	mux.HandleFunc("/ws", newWebsocketHandler(newConn, doneConn))
	mux.HandleFunc("/notifications", newNotificationsHandler(notifCache))

	// Create and start HTTP server
	httpServer := &http.Server{Addr: ":3000", Handler: mux}
	err := httpServer.ListenAndServe()
	if err != nil {
		return nil, err
	}

	return httpServer, nil
}
