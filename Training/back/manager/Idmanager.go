package manager

import (
	"fmt"
	"sync"
	"time"
)

type IdManager interface {
	GetId() string
	InsertId(Id int32)
}

type DefaultIdManager struct {
	Number []int32
	mux    *sync.RWMutex
}

func NewDefaultIdManager() *DefaultIdManager {
	m := make([]int32, 100)
	for i := 0; i < 100; i++ {
		m[i] = int32(i)
	}
	return &DefaultIdManager{
		Number: m,
		mux:    &sync.RWMutex{},
	}
}

func (h *DefaultIdManager) GetId() string {
	h.mux.RLock()
	defer h.mux.RUnlock()
	id := h.Number[0]
	h.Number = h.Number[1:]
	str := fmt.Sprint(time.Now().Format("2006-01-02 15:04:05"), id)
	return str
}

func (h *DefaultIdManager) InsertId(id int32) {
	h.mux.Lock()
	defer h.mux.Unlock()
	h.Number = append(h.Number, id)
}
