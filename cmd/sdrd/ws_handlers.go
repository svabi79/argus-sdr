package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"sdr-wideband-suite/internal/recorder"
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

		// Parse query params for remote clients: ?binary=1&bins=2048&fps=5
		q := r.URL.Query()
		c := &client{conn: conn, send: make(chan []byte, 64), done: make(chan struct{})}
		if q.Get("binary") == "1" || q.Get("binary") == "true" {
			c.binary = true
		}
		if v, err := strconv.Atoi(q.Get("bins")); err == nil && v > 0 {
			c.maxBins = v
		}
		if v, err := strconv.Atoi(q.Get("fps")); err == nil && v > 0 {
			c.targetFps = v
			// frameSkip: if server runs at ~15fps and client wants 5fps → skip 3
			c.frameSkip = 15 / v
			if c.frameSkip < 1 {
				c.frameSkip = 1
			}
		}

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
					// Binary frames can be large (130KB+) — need more time
					deadline := 500 * time.Millisecond
					if !c.binary {
						deadline = 200 * time.Millisecond
					}
					_ = conn.SetWriteDeadline(time.Now().Add(deadline))
					msgType := websocket.TextMessage
					if c.binary {
						msgType = websocket.BinaryMessage
					}
					if err := conn.WriteMessage(msgType, msg); err != nil {
						return
					}
				case <-ping.C:
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				case <-c.done:
					return
				}
			}
		}()
		// Read loop: handle config messages from client + keep-alive
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Try to parse as client config update
			var cfg struct {
				Binary *bool `json:"binary,omitempty"`
				Bins   *int  `json:"bins,omitempty"`
				FPS    *int  `json:"fps,omitempty"`
			}
			if json.Unmarshal(msg, &cfg) == nil {
				if cfg.Binary != nil {
					c.binary = *cfg.Binary
				}
				if cfg.Bins != nil && *cfg.Bins > 0 {
					c.maxBins = *cfg.Bins
				}
				if cfg.FPS != nil && *cfg.FPS > 0 {
					c.targetFps = *cfg.FPS
					c.frameSkip = 15 / *cfg.FPS
					if c.frameSkip < 1 {
						c.frameSkip = 1
					}
				}
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

		// LL-3: Subscribe BEFORE upgrading WebSocket.
		// SubscribeAudio now returns AudioInfo and never immediately closes
		// the channel — it queues pending listeners instead.
		subID, ch, audioInfo, err := streamer.SubscribeAudio(freq, bw, mode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
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

		// LL-2: Send actual audio info (channels, sample rate from session)
		info := map[string]any{
			"type":        "audio_info",
			"sample_rate": audioInfo.SampleRate,
			"channels":    audioInfo.Channels,
			"format":      audioInfo.Format,
			"demod":       audioInfo.DemodName,
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
			case data, ok := <-ch:
				if !ok {
					log.Printf("ws/audio: stream ended freq=%.1fMHz", freq/1e6)
					return
				}
				if len(data) == 0 {
					continue
				}
				_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				// Tag protocol: first byte is message type
				//   0x00 = AudioInfo JSON (send as TextMessage, strip tag)
				//   0x01 = PCM audio (send as BinaryMessage, strip tag)
				tag := data[0]
				payload := data[1:]
				msgType := websocket.BinaryMessage
				if tag == 0x00 {
					msgType = websocket.TextMessage
				}
				if err := conn.WriteMessage(msgType, payload); err != nil {
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
