package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func newWebsocketHandler(newConn socketChan, doneConn socketChan) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			host := strings.Split(r.Host, ":")
			return host[0] == "localhost"
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
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
}

func newNotificationsHandler(notifCache *notificationCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notifs := notifCache.values()
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
