package cache

import (
	"fmt"
	"sync"
)

//Error Codes for keystates
var (
	ErrInvalidKey         = fmt.Errorf("found invalid key")
	ErrItemNotPresent     = fmt.Errorf("item not present")
	ErrKeyStateNotPresent = fmt.Errorf("keystate datastructure not initialized")
)

//keyState structure
type keyStates struct {
	keyState map[string]state //states{INPROCESS,DONE,INVALID,NOPRESENT}
	mu       *sync.RWMutex    //RWmutex
}

type state int

//states declared in constants
const (
	INPROCESS state = iota
	DONE
	INVALID
	NOPRESENT
)

var ks keyStates

//initializing ks keyStates
func init() {
	ks.keyState = make(map[string]state)
	ks.mu = &sync.RWMutex{}
}

//Set status/state with respective key in keyState
func (ks *keyStates) Set(key string, status state) (bool, error) {
	ok := false
	if ks != nil && ks.keyState != nil {
		ks.mu.Lock()
		ok = ks.set(key, status)
		ks.mu.Unlock()
	} else {
		return false, ErrKeyStateNotPresent
	}
	if !ok {
		return false, ErrInvalidKey
	}
	return true, nil
}

//utility set used in Set
func (ks *keyStates) set(k string, s state) bool {
	if len(k) <= 0 {
		return false
	}
	ks.keyState[k] = s
	return true
}

//Get status/state of respective key in keyState
func (ks *keyStates) Get(key string) (state, error) {
	var (
		status state
		ok     bool
	)
	if !isValid(key) {
		return INVALID, ErrInvalidKey
	}
	if ks != nil && ks.keyState != nil {
		status, ok = ks.get(key)
	} else {
		return INVALID, ErrKeyStateNotPresent
	}

	if !ok {
		return NOPRESENT, ErrItemNotPresent
	}

	return status, nil
}

//utility get for Get
func (ks *keyStates) get(k string) (state, bool) {
	item, found := ks.keyState[k]
	if !found {
		return NOPRESENT, false
	}
	return item, true
}

//checkKeyvalidity
func isValid(key string) bool {
	if len(key) > 0 {
		return true
	} else {
		return false
	}
}
