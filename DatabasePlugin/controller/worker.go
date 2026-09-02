package controller

import (
	"context"
	"database/errortable"
	"database/handler"
	"database/manager"
	"database/pkg"
	"database/report"
	"database/send"
	"errors"
	"os"
	"sync"
	"time"
)

type Worker interface {
	Work(int, send.Sender, handler.Handler, manager.Manager) error
}

type DefaultWorker struct {
	ctx      context.Context
	TaskChan chan handler.HandleTask
	Id       string
	mux      *sync.RWMutex
	file     *os.File
	reporter report.Reporter
	et       errortable.ErrorTable
}

func NewDefaultWorker(ctx context.Context, tc chan handler.HandleTask, id string, mux *sync.RWMutex, file *os.File, reporter report.Reporter, et errortable.ErrorTable) *DefaultWorker {
	return &DefaultWorker{
		ctx:      ctx,
		TaskChan: tc,
		Id:       id,
		mux:      mux,
		file:     file,
		reporter: reporter,
		et:       et,
	}
}

func (w *DefaultWorker) Work(wid int, s send.Sender, h handler.Handler, m manager.Manager) error {
	var timer = time.NewTimer(time.Second * 10)
	for {
		select {
		case <-w.ctx.Done():
			return errors.New("manual shutdown")
		case task := <-w.TaskChan:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(time.Second * 10)
			ss, errorCount, err := h.Handle(task)
			if errorCount > 0 && w.countErrors(s, m, errorCount, err) {
				if err != nil {
					return err
				}
				return errors.New("error rate reached threshold")
			}
			if len(ss.Inputs) > 0 {
				err = s.SendToTrain(&ss)
				if err != nil {
					if errors.Is(err, send.ErrTaskStopped) {
						_ = m.Stop(w.Id)
						return nil
					}
					pkg.MuxLog(w.file, err, w.Id, false, w.mux)
					if w.countErrors(s, m, 1, err) {
						return err
					}
					continue
				}
			}
		case <-timer.C:
			SingleList <- wid
			err := s.Finish()
			if err != nil {
				if errors.Is(err, send.ErrTaskStopped) {
					_ = m.Stop(w.Id)
					return nil
				}
				pkg.MuxLog(w.file, err, w.Id, false, w.mux)
				if w.reporter != nil {
					w.reporter.Report("controller", err.Error(), w.Id)
				}
				_ = s.ReportError(err.Error())
				_ = m.Stop(w.Id)
				return err
			}
			m.Pop(w.Id)
			return nil
		}
	}
}

func (w *DefaultWorker) countErrors(s send.Sender, m manager.Manager, n int64, err error) bool {
	reached, e := w.et.AddErrors(w.Id, n)
	if e == nil && reached {
		msg := "error rate reached 40%"
		if err != nil {
			msg = err.Error()
		}
		if w.reporter != nil {
			w.reporter.Report("controller", msg, w.Id)
		}
		_ = s.ReportError(msg)
		_ = m.Stop(w.Id)
		return true
	}
	return false
}
