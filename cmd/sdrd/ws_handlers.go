package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"sdr-visual-suite/internal/recorder"
)

func registerWSHandlers(mux *http.ServeMux, h *hub, recMgr *recorder.Manager) {
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

	// /ws/audio — WebSocket endpoint for continuous live-listen audio streaming.
	// Client connects with query params: freq, bw, mode
	// Server sends binary frames of PCM s16le audio at 48kHz.
	mux.HandleFunc("/ws/audio", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		freq, _ := strconv.ParseFloat(q.Get("freq"), 64)
		bw, _ := strconv.ParseFloat(q.Get("bw"), 64)
		mode := q.Get("mode")
		if freq <= 0 {
			http.Error(w, "freq required", http.StatusBadRequest)
			return
		}
		if bw <= 0 {
			bw = 12000
		}

		streamer := recMgr.StreamerRef()
		if streamer == nil {
			http.Error(w, "streamer not available", http.StatusServiceUnavailable)
			return
		}

		subID, ch := streamer.SubscribeAudio(freq, bw, mode)
		if ch == nil {
			http.Error(w, "no active stream for this frequency", http.StatusNotFound)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			streamer.UnsubscribeAudio(subID)
			log.Printf("ws/audio upgrade failed: %v", err)
			return
		}
		defer func() {
			streamer.UnsubscribeAudio(subID)
			_ = conn.Close()
		}()

		log.Printf("ws/audio: client connected freq=%.1fMHz mode=%s", freq/1e6, mode)

		// Send audio stream info as first text message
		info := map[string]any{
			"type":        "audio_info",
			"sample_rate": 48000,
			"channels":    1,
			"format":      "s16le",
			"freq":        freq,
			"mode":        mode,
		}
		if infoBytes, err := json.Marshal(info); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, infoBytes)
		}

		// Read goroutine (to detect disconnect)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()

		ping := time.NewTicker(30 * time.Second)
		defer ping.Stop()

		for {
			select {
			case pcm, ok := <-ch:
				if !ok {
					log.Printf("ws/audio: stream ended freq=%.1fMHz", freq/1e6)
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				if err := conn.WriteMessage(websocket.BinaryMessage, pcm); err != nil {
					log.Printf("ws/audio: write error: %v", err)
					return
				}
			case <-ping.C:
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				log.Printf("ws/audio: client disconnected freq=%.1fMHz", freq/1e6)
				return
			}
		}
	})
}
