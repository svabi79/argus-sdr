package main

import "sync"

type phaseSnapshot struct {
	mu    sync.RWMutex
	state phaseState
}

func (p *phaseSnapshot) Set(state phaseState) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.state = state
	p.mu.Unlock()
}

func (p *phaseSnapshot) Snapshot() phaseState {
	if p == nil {
		return phaseState{}
	}
	p.mu.RLock()
	state := p.state
	p.mu.RUnlock()
	return state
}
