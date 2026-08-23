package controller

import (
	"context"
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
}

func NewDefaultWorker(ctx context.Context, tc chan handler.HandleTask, id string, mux *sync.RWMutex, file *os.File, reporter report.Reporter) *DefaultWorker {
	return &DefaultWorker{
		ctx:      ctx,
		TaskChan: tc,
		Id:       id,
		mux:      mux,
		file:     file,
		reporter: reporter,
	}
}

func (w *DefaultWorker) Work(wid int, s send.Sender, h handler.Handler, m manager.Manager) error {
	var timer = time.NewTimer(time.Second * 10)
	for {
		select {
		case <-w.ctx.Done():
			return errors.New("manual shutdown")
		case task := <-w.TaskChan:
			ss, err := h.Handle(task)
			if err != nil {
				pkg.MuxLog(w.file, err, w.Id, false, w.mux)
				if w.reporter != nil {
					w.reporter.Report("controller", err.Error(), w.Id)
				}
				_ = s.ReportError(err.Error())
				return err
			}
			err = s.SendToTrain(&ss)
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
				return err
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(time.Second * 10)
		case <-timer.C:
			SingleList <- wid
			err := s.Finish()
			if err != nil {
				if errors.Is(err, send.ErrTaskStopped) {
					_ = m.Stop(w.Id)
					return nil
				}
				for i := 0; i < 5; i++ {
					time.Sleep(time.Second * 10)
					err = s.Finish()
					if err == nil {
						return nil
					}
					if errors.Is(err, send.ErrTaskStopped) {
						_ = m.Stop(w.Id)
						return nil
					}
				}
				pkg.MuxLog(w.file, err, w.Id, false, w.mux)
				if w.reporter != nil {
					w.reporter.Report("controller", err.Error(), w.Id)
				}
				_ = s.ReportError(err.Error())
				return err
			}
			m.Pop(w.Id)
			return nil
		}
	}
}
