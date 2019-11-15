package api

import (
	"github.com/gcash/bchd/snowglobe/models"
	"github.com/gorilla/websocket"
)

type emptyChan chan (struct{})
type socketChan chan (*websocket.Conn)

type avaCallbackFn func(vr *models.VoteRecord)

type notif struct {
	VertexHash       string `json:"vertex_hash"`
	VertexType       string `json:"vertex_type"`
	FinalizationTime string `json:"finalization_time"`
}
