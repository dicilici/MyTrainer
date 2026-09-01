package manager

import (
	"errors"
	"sync"
	"time"
	"train/back/Database"
	"train/back/sender"
	"train/config"
)

type Element struct {
	C         config.Config
	S         sender.Client
	D         Database.DatabaseHandler
	StartTime time.Time
}

type Manager interface {
	Get(id string) (Element, error)
	Insert(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error
	Pop(id string) error
	Cover(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error
	GetMap() map[string]Element
	GetNumber() int
	Close()
}

type DefaultManager struct {
	List map[string]Element
	mux  *sync.RWMutex
}

func NewDefaultManager() *DefaultManager {
	return &DefaultManager{
		List: make(map[string]Element, 10),
		mux:  &sync.RWMutex{},
	}
}

func (h *DefaultManager) GetNumber() int {
	h.mux.RLock()
	defer h.mux.RUnlock()
	num := len(h.List)
	return num
}

func (h *DefaultManager) GetMap() map[string]Element {
	h.mux.RLock()
	defer h.mux.RUnlock()
	m := make(map[string]Element, len(h.List))
	for k, v := range h.List {
		m[k] = v
	}
	return m
}

func (h *DefaultManager) Insert(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error {
	h.mux.Lock()
	defer h.mux.Unlock()
	if _, ok := h.List[id]; ok {
		return errors.New("this ID has already been taken")
	}
	h.List[id] = Element{C: c, S: s, D: d, StartTime: time.Now()}
	return nil
}

func (h *DefaultManager) Pop(id string) error {
	h.mux.Lock()
	defer h.mux.Unlock()
	if _, ok := h.List[id]; !ok {
		return errors.New("this ID has not been added to the list")
	}
	delete(h.List, id)
	return nil
}

func (h *DefaultManager) Cover(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error {
	h.mux.Lock()
	defer h.mux.Unlock()
	old, ok := h.List[id]
	if !ok {
		return errors.New("this ID has not been added to the list")
	}
	h.List[id] = Element{C: c, S: s, D: d, StartTime: old.StartTime}
	return nil
}

func (h *DefaultManager) Get(id string) (Element, error) {
	h.mux.RLock()
	defer h.mux.RUnlock()
	if _, ok := h.List[id]; !ok {
		return Element{}, errors.New("this ID has not been added to the list")
	}
	return h.List[id], nil
}

func (h *DefaultManager) Close() {
	for k, v := range h.List {
		v.S.Disconnect()
		v.D.Disconnect()
		h.Pop(k)
	}
}
