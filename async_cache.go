package cache

import (
	"sync"
)

//status is signal which helps to identify in which state the current process with a specific key is
type status int

//status for keys/process declared in constants
const (
	STATUS_NOTPRESENT            status = iota //current key is not present in KeyMap
	STATUS_INPROCESS                           // current key/process is already INPROCESS to fetch data from data source
	STATUS_DONE                                //current key/process have DONE fetching data and updated in cache
	STATUS_STATUS_INTERNAL_ERROR               //current key/process recieved internal_error while fetching data
	STATUS_INVALID_KEY                         // current key is invalid to be fetched
)

type keyStatus struct {
	keyMap map[string]status //status{INPROCESS,DONE,INVALID,NOPRESENT,INVALID_KEY}
	mu     *sync.RWMutex
}

//To Create A New keyStatus
func NewKeyStatus() *keyStatus {
	return &keyStatus{
		keyMap: make(map[string]status),
		mu:     &sync.RWMutex{},
	}
}

//Set status/status with respective key in keyStatus
func (ks *keyStatus) Set(key string, status status) {

	if len(key) > 0 {
		ks.mu.Lock()
		ks.keyMap[key] = status //updating status in keyMap for particular "key"
		ks.mu.Unlock()
	}
}

//Get status/status of respective key in keyStatus
func (ks *keyStatus) Get(key string) status {
	if len(key) == 0 {
		return STATUS_INVALID_KEY
	}
	ks.mu.RLock()
	status, found := ks.keyMap[key]
	ks.mu.RUnlock()
	if !found {
		return STATUS_NOTPRESENT
	}
	return status

}
