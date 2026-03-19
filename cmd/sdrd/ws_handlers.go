package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func registerWSHandlers(mux *http.ServeMux, h *hub) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "null" {
			return true
		}
		return true
	}}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade failed: %v (origin: %s)", err, r.Header.Get("Origin"))
			return
		}
		c := &client{conn: conn, send: make(chan []byte, 32), done: make(chan struct{})}
		h.add(c)
		defer func() {
			h.remove(c)
			_ = conn.Close()
		}()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		go func() {
			ping := time.NewTicker(30 * time.Second)
			defer ping.Stop()
			for {
				select {
				case msg, ok := <-c.send:
					if !ok {
						return
					}
					_ = conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						return
					}
				case <-ping.C:
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						log.Printf("ws ping error: %v", err)
						return
					}
				}
			}
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
}
