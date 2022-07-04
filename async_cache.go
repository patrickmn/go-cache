package cache

import (
	"fmt"
	"sync"
)

type KeyState map[string]state

// type keyState struct {
// 	ks KeyState
// }

var (
	m sync.RWMutex
)

type state int

const (
	INPROCESS state = iota
	DONE
	INVALID
)

func getState(s state) {
	fmt.Println("state", s)
}

// func NewAsyncCache() keyState {
// 	var ks keyState
// 	ks = keyState{ks: make(map[string]state)
// 	}
// 	return ks
// }

func (ks KeyState) Set(key string, S state) bool {
	getState(S)
	if len(ks) == 0 {
		ks = make(KeyState)
	}
	if len(key) > 0 {
		m.Lock()
		ks[key] = S
		m.Unlock()
	} else {
		return false
	}
	return true
}

func (ks KeyState) Get(key string) state {
	if len(key) > 0 {
		currS := ks[key]
		return currS
	} else {
		return INVALID
	}
}
