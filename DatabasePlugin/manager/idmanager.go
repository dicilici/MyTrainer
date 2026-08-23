package manager

import (
	"database/pkg"
	"errors"
	"os"
	"sync"
)

type IdManager interface {
	SelectPop(string) error
	Pop() error
	Insert(string) error
	Get() ([]string, error)
}

type DefaultIdManager struct {
	List   []string
	Number int
	Top    int
	mu     sync.RWMutex
	mux    *sync.RWMutex
	file   *os.File
}

func NewDefaultIdManager(mux *sync.RWMutex, file *os.File) *DefaultIdManager {
	return &DefaultIdManager{
		List:   make([]string, 0, 100),
		Number: -1,
		Top:    -1,
		mu:     sync.RWMutex{},
		mux:    mux,
		file:   file,
	}
}

func (m *DefaultIdManager) Pop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Number == -1 {
		err := errors.New("no ID in the queue")
		pkg.MuxLog(m.file, err, "-3", false, m.mux)
		return err
	}
	m.List = m.List[1:]
	m.Number--
	m.Top--
	return nil
}

func (m *DefaultIdManager) Insert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.List = append(m.List, id)
	m.Number++
	return nil
}

func (m *DefaultIdManager) Get() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Top == m.Number {
		return nil, nil
	} else if m.Number == -1 {
		err := errors.New("no ID in the queue")
		pkg.MuxLog(m.file, err, "-3", false, m.mux)
		return nil, err
	}
	var re []string = make([]string, 0)
	for i := m.Top + 1; i <= m.Number; i++ {
		re = append(re, m.List[i])
	}
	m.Top = m.Number
	return re, nil
}

func (m *DefaultIdManager) SelectPop(id string) error {
	var Find = false
	for index, i := range m.List {
		if i == id {
			Find = true
			m.mu.Lock()
			defer m.mu.Unlock()
			m.List = append(m.List[:index], m.List[index+1:]...)
			m.Number--
			m.Top--
		}
	}
	if Find == false {
		err := errors.New("no ID in the queue")
		pkg.MuxLog(m.file, err, id, false, m.mux)
		return err
	}
	return nil
}
