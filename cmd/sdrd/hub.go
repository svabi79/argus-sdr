package main

import (
	"encoding/json"
	"log"
	"time"

	"sdr-visual-suite/internal/detector"
)

func (s *signalSnapshot) set(sig []detector.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals = append([]detector.Signal(nil), sig...)
}

func (s *signalSnapshot) get() []detector.Signal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]detector.Signal(nil), s.signals...)
}

func (g *gpuStatus) set(active bool, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Active = active
	if err != nil {
		g.Error = err.Error()
	} else {
		g.Error = ""
	}
}

func (g *gpuStatus) snapshot() gpuStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return gpuStatus{Available: g.Available, Active: g.Active, Error: g.Error}
}

func newHub() *hub {
	return &hub{clients: map[*client]struct{}{}, lastLogTs: time.Now()}
}

func (h *hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	log.Printf("ws connected (%d clients)", len(h.clients))
}

func (h *hub) remove(c *client) {
	c.closeOnce.Do(func() { close(c.done) })
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	log.Printf("ws disconnected (%d clients)", len(h.clients))
}

func (h *hub) broadcast(frame SpectrumFrame) {
	b, err := json.Marshal(frame)
	if err != nil {
		log.Printf("marshal frame: %v", err)
		return
	}

	h.mu.Lock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		select {
		case c.send <- b:
		default:
			h.remove(c)
		}
	}
	h.frameCnt++
	if time.Since(h.lastLogTs) > 2*time.Second {
		h.lastLogTs = time.Now()
		log.Printf("broadcast frames=%d clients=%d", h.frameCnt, len(clients))
	}
}
