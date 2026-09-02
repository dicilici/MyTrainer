package errortable

import (
	"errors"
	"sync"
)

type Record struct {
	Id        string // 任务 id
	Threshold int64  // 触发错误的条数（上取整）
	Occurred  int64  // 已发生的错误条数
}

type ErrorTable interface {
	Insert(id string, threshold int64) error
	AddErrors(id string, n int64) (bool, error)
	Remove(id string) error
}

type DefaultErrorTable struct {
	records map[string]*Record
	mux     *sync.Mutex
}

func NewDefaultErrorTable() *DefaultErrorTable {
	return &DefaultErrorTable{
		records: make(map[string]*Record),
		mux:     &sync.Mutex{},
	}
}

func (t *DefaultErrorTable) Insert(id string, threshold int64) error {
	t.mux.Lock()
	defer t.mux.Unlock()
	if _, ok := t.records[id]; ok {
		return errors.New("record already exists")
	}
	t.records[id] = &Record{Id: id, Threshold: threshold}
	return nil
}

func (t *DefaultErrorTable) AddErrors(id string, n int64) (bool, error) {
	t.mux.Lock()
	defer t.mux.Unlock()
	r, ok := t.records[id]
	if !ok {
		return false, errors.New("record not found")
	}
	r.Occurred += n
	if r.Occurred >= r.Threshold {
		delete(t.records, id)
		return true, nil
	}
	return false, nil
}

func (t *DefaultErrorTable) Remove(id string) error {
	t.mux.Lock()
	defer t.mux.Unlock()
	delete(t.records, id)
	return nil
}
