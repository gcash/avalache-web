package main

import (
	"log"
	"net/http"

	sherpa "github.com/gcash/avalanche-web"
	"golang.org/x/net/websocket"
	// "github.com/gorilla/websocket"
)

func main() {
	sherpa.New()

	notifications = &limitMap{
		notifications: make(map[int]Notif),
		limit:         8,
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
