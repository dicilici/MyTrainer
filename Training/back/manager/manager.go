package manager

import (
	"errors"
	"sync"
	"train/back/Database"
	"train/back/sender"
	"train/config"
)

type Manager interface {
	Get(id string) ([]interface{}, error)
	Insert(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error
	Pop(id string) error
	Cover(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error
	GetMap() map[string][]interface{}
	GetNumber() int
	Close()
}
type DefaultManager struct {
	List map[string][]interface{}
	mux  *sync.RWMutex
}

func NewDefaultManager() *DefaultManager {
	return &DefaultManager{
		List: make(map[string][]interface{}, 10),
		mux:  &sync.RWMutex{},
	}
}

func (h *DefaultManager) GetNumber() int {
	h.mux.RLock()
	defer h.mux.RUnlock()
	num := len(h.List)
	return num
}

func (h *DefaultManager) GetMap() map[string][]interface{} {
	h.mux.RLock()
	defer h.mux.RUnlock()
	m := make(map[string][]interface{}, len(h.List))
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
	} else {
		i := []interface{}{c, s, d}
		h.List[id] = i
	}
	return nil
}

func (h *DefaultManager) Pop(id string) error {
	h.mux.Lock()
	defer h.mux.Unlock()
	if _, ok := h.List[id]; !ok {
		return errors.New("this ID has not been added to the list")
	} else {
		delete(h.List, id)
	}
	return nil
}

func (h *DefaultManager) Cover(id string, c config.Config, s sender.Client, d Database.DatabaseHandler) error {
	h.mux.Lock()
	defer h.mux.Unlock()
	if _, ok := h.List[id]; !ok {
		return errors.New("this ID has not been added to the list")
	} else {
		i := []interface{}{c, s, d}
		h.List[id] = i
	}
	return nil
}

func (h *DefaultManager) Get(id string) ([]interface{}, error) {
	h.mux.RLock()
	defer h.mux.RUnlock()
	if _, ok := h.List[id]; !ok {
		return []interface{}{}, errors.New("this ID has not been added to the list")
	}
	return h.List[id], nil
}

func (h *DefaultManager) Close() {
	for k, v := range h.List {
		v[1].(sender.Client).Disconnect()
		v[2].(Database.DatabaseHandler).Disconnect()
		h.Pop(k)
	}
}
