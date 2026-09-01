package manager

import (
	"context"
	"database/config"
	"database/errortable"
	"database/handler"
	"database/pkg"
	"database/report"
	"database/send"
	"errors"
	"os"
	"sync"
)

type Manager interface {
	GetNumber() int
	Get(id string) (Element, error)
	Insert(string, Element) error
	Pop(id string) error
	GetAll() []Element
	Stop(id string) error
}

type Element struct {
	C      config.Config
	S      send.Sender
	H      handler.Handler
	R      report.Reporter
	Ctx    context.Context
	Cancel context.CancelFunc
}

type DefaultManager struct {
	relate map[string]Element
	mu     sync.RWMutex
	mux    *sync.RWMutex
	file   *os.File
	et     errortable.ErrorTable
}

func NewDefaultManager(mux *sync.RWMutex, file *os.File, et errortable.ErrorTable) *DefaultManager {
	return &DefaultManager{
		relate: make(map[string]Element, 10),
		mu:     sync.RWMutex{},
		mux:    mux,
		file:   file,
		et:     et,
	}
}

func (m *DefaultManager) GetNumber() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.relate)
}

func (m *DefaultManager) Get(id string) (Element, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.relate[id]; ok {
		return v, nil
	}
	return Element{}, errors.New("Not Found")
}

func (m *DefaultManager) Insert(id string, element Element) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.relate[id]
	if ok == true {
		err := errors.New("Already exists")
		pkg.MuxLog(m.file, err, id, false, m.mux)
		return err
	} else {
		m.relate[id] = element
	}
	return nil
}

func (m *DefaultManager) Pop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.relate[id]
	if ok == false {
		err := errors.New("Not exists")
		pkg.MuxLog(m.file, err, id, false, m.mux)
		return err
	}
	delete(m.relate, id)
	m.et.Remove(id)
	return nil
}

func (m *DefaultManager) GetAll() []Element {
	m.mu.RLock()
	defer m.mu.RUnlock()
	elements := make([]Element, 0, len(m.relate))
	for _, element := range m.relate {
		elements = append(elements, element)
	}
	return elements
}

func (m *DefaultManager) Stop(id string) error {
	m.mu.RLock()
	e, ok := m.relate[id]
	m.mu.RUnlock()
	if !ok {
		return errors.New("Not exists")
	}
	if e.Cancel != nil {
		e.Cancel()
	}
	m.et.Remove(id)
	return nil
}
