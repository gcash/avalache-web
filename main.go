package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchutil"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type Notif struct {
	Txid string `json:"txid"`
	FinalizationTIme string `json:"finalizationTime"`
}

type limitMap struct {
	limit int
	count int
	notifications map[int]Notif
}

func (m *limitMap) Append(n Notif) {
	var toDelete []int
	if len(m.notifications) >= m.limit {
		for i := range m.notifications {
			if i < m.count + 1 - m.limit {
				toDelete = append(toDelete, i)
			}
		}
	}
	for _, i := range toDelete {
		delete(m.notifications, i)
	}
	m.notifications[m.count+1] = n
	m.count++
}

func (m *limitMap) Notifications() []Notif {
	var notifs []Notif
	for _, n := range m.notifications {
		notifs = append(notifs, n)
	}
	return notifs
}

var newConn chan *websocket.Conn
var doneConn chan *websocket.Conn
var notifications *limitMap

func main() {
	notifications = &limitMap{
		notifications: make(map[int]Notif),
		limit: 8,
	}

	notifChan := make(chan Notif)
	newConn = make(chan *websocket.Conn)
	doneConn = make(chan *websocket.Conn)
	if err := connectBchdWebsocket(notifChan); err != nil {
		log.Fatal(err)
	}
	go listenChans(newConn, notifChan)
	http.HandleFunc("/ws", handleWebsocket)
	http.HandleFunc("/notifications", handleNotifications)

	http.Handle("/", http.StripPrefix("/", http.HandlerFunc(static_handler)))

	log.Println("Listening...")
	http.ListenAndServe(":3000", nil)
}

// generate with: go-bindata -prefix "static/" -pkg main -o bindata.go static/...
func static_handler(rw http.ResponseWriter, req *http.Request) {
	var path string = req.URL.Path
	if path == "" {
		path = "index.html"
	}
	if bs, err := Asset(path); err != nil {
		rw.WriteHeader(http.StatusNotFound)
	} else {
		var reader = bytes.NewBuffer(bs)
		io.Copy(rw, reader)
	}
}

func listenChans(newConn chan *websocket.Conn, newNotif chan Notif) {
	conns := make(map[*websocket.Conn]struct{})
	for {
		select {
		case conn := <- newConn:
			conns[conn] = struct{}{}
		case conn := <- doneConn:
			delete(conns, conn)
		case n := <- newNotif:
			notifications.Append(n)
			out, err := json.MarshalIndent(&n, "", "    ")
			if err != nil {
				log.Println(err)
				continue
			}
			for conn := range conns {
				conn.WriteMessage(websocket.TextMessage, out)
			}
		}
	}
}

func connectBchdWebsocket(notificationChan chan Notif) error {
	// Only override the handlers for notifications you care about.
	// Also note most of these handlers will only be called if you register
	// for notifications.  See the documentation of the rpcclient
	// NotificationHandlers type for more details about each handler.
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnTxFinalized: func(txid *chainhash.Hash, finalizationTime time.Duration) {
			notificationChan <- Notif{txid.String(), finalizationTime.String()}
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
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		host := strings.Split(r.Host, ":")
		if host[0] == "localhost" {
			return true
		}
		return false
	},
}

//
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	newConn <- c

	go func() {
		defer c.Close()
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				doneConn <- c
				break
			}
		}
	}()
}

// GET /notifications. Returns a JSON list of the last 8 notifications
func handleNotifications(w http.ResponseWriter, r *http.Request) {
	notifs := notifications.Notifications()
	out, err := json.MarshalIndent(&notifs, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(notifs) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fmt.Fprint(w, string(out))
}