package controller

import (
	"context"
	"database/handler"
	"database/manager"
	"database/pkg"
	Database "database/receive"
	"errors"
	"os"
	"sync"
)

var SingleList chan int = make(chan int, 10)

const pageSize = 100

type Controller interface {
	Run(*sync.WaitGroup) error
	Begin([]string) error
	Stop() error
	Pop(int) error
	Get() (int, bool)
}

type DefaultController struct {
	Idm      manager.IdManager
	M        manager.Manager
	TaskList []chan handler.HandleTask
	ctx      context.Context
	Top      int
	mux      *sync.RWMutex
	file     *os.File
}

func NewDefaultController(ctx context.Context, idm manager.IdManager, m manager.Manager, mux *sync.RWMutex, file *os.File) *DefaultController {
	var channelList []chan handler.HandleTask = make([]chan handler.HandleTask, 0, 10)
	for index, _ := range channelList {
		channelList[index] = make(chan handler.HandleTask, 15)
	}
	return &DefaultController{
		Idm:      idm,
		M:        m,
		TaskList: channelList,
		ctx:      ctx,
		Top:      -1,
		mux:      mux,
		file:     file,
	}
}

func (m *DefaultController) Begin(tasks []string) error {
	for _, task := range tasks {
		e, err := m.M.Get(task)
		if err != nil {
			pkg.MuxLog(m.file, err, task, false, m.mux)
			if e.R != nil {
				e.R.Report("controller", err.Error(), task)
			}
			return err
		}
		cursor := 0
		stopped := false
		for !stopped {
			ht, newCursor, err := e.H.Get(cursor, pageSize)
			if err != nil {
				pkg.MuxLog(m.file, err, task, false, m.mux)
				if e.R != nil {
					e.R.Report("controller", err.Error(), task)
				}
				return err
			}
			cursor = newCursor
			if len(ht) == 0 {
				break
			}
			for _, handleTask := range ht {
				if e.Ctx != nil {
					select {
					case <-e.Ctx.Done():
						stopped = true
					default:
					}
				}
				if stopped {
					break
				}
				select {
				case <-m.ctx.Done():
					return errors.New("manual shutdown")
				case idCancel := <-SingleList:
					for ta := range m.TaskList[idCancel] {
						id, find := m.Get()
						if find == false {
							w := NewDefaultWorker(m.ctx, m.TaskList[id], task, m.mux, m.file, e.R)
							go w.Work(id, e.S, e.H, m.M)
						}
						m.TaskList[id] <- ta
					}
					err = m.Pop(idCancel)
					if err != nil {
						pkg.MuxLog(m.file, err, "-3", false, m.mux)
						if e.R != nil {
							e.R.Report("controller", err.Error(), "-3")
						}
						return err
					}
				default:
					id, find := m.Get()
					if find == false {
						w := NewDefaultWorker(m.ctx, m.TaskList[id], task, m.mux, m.file, e.R)
						go w.Work(id, e.S, e.H, m.M)
					}
					m.TaskList[id] <- handleTask
				}
			}
		}
		if stopped {
			m.M.Pop(task)
			m.Idm.SelectPop(task)
			_ = e.S.Disconnect()
			_ = e.H.DisConnect()
		}
	}
	return nil
}

func (c *DefaultController) Run(wg *sync.WaitGroup) error {
	for {
		select {
		case <-c.ctx.Done():
			wg.Done()
			return errors.New("manual shutdown")
		case <-Database.TaskCome:
			tasks, err := c.Idm.Get()
			if err != nil {
				pkg.MuxLog(c.file, err, "-3", false, c.mux)
				elements := c.M.GetAll()
				if len(elements) > 0 && elements[0].R != nil {
					elements[0].R.Report("controller", err.Error(), "-3")
				}
				wg.Done()
				return err
			}
			go c.Begin(tasks)
		}
	}
}

func (c *DefaultController) Get() (int, bool) {
	for i := 0; i <= c.Top; i++ {
		if len(c.TaskList[i]) <= 10 {
			return i, true
		}
	}
	c.TaskList = append(c.TaskList, make(chan handler.HandleTask, 15))
	c.Top++
	return c.Top, false
}

func (m *DefaultController) Pop(id int) error {
	if m.Top < id {
		err := errors.New("id is out of range")
		pkg.MuxLog(m.file, err, "-3", false, m.mux)
		return err
	}
	close(m.TaskList[id])
	m.TaskList = append(m.TaskList[:id], m.TaskList[id+1:]...)
	return nil
}

func (m *DefaultController) Stop() error {
	for _, task := range m.TaskList {
		close(task)
	}
	close(SingleList)
	for _, e := range m.M.GetAll() {
		err := e.S.Disconnect()
		if err != nil {
			pkg.MuxLog(m.file, err, "-3", false, m.mux)
			if e.R != nil {
				e.R.Report("controller", err.Error(), "-3")
			}
			return err
		}
		err = e.H.DisConnect()
		if err != nil {
			pkg.MuxLog(m.file, err, "-3", false, m.mux)
			if e.R != nil {
				e.R.Report("controller", err.Error(), "-3")
			}
			return err
		}
	}
	return nil
}
