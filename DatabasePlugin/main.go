package main

import (
	"context"
	"database/controller"
	"database/errortable"
	"database/manager"
	Database "database/receive"
	"os"
	"sync"
	"time"
)

var r Database.Receiver
var m manager.Manager
var idm manager.IdManager
var c controller.Controller
var LogPath string
var ctx context.Context
var cancel context.CancelFunc
var wg *sync.WaitGroup

func init() {
	ctx, cancel = context.WithCancel(context.Background())
	wg = &sync.WaitGroup{}
	LogPath = os.Getenv("DATA_LOG")
	file, _ := os.OpenFile(LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	mux := &sync.RWMutex{}
	idm = manager.NewDefaultIdManager(mux, file)
	et := errortable.NewDefaultErrorTable()
	m = manager.NewDefaultManager(mux, file, et)
	r = Database.NewDefaultReceiver(LogPath, m, idm, ctx, mux, file, et)
	c = controller.NewDefaultController(ctx, idm, m, mux, file, et)
}

func main() {
	go c.Run(wg)
	go r.Run(wg)
	for {
		select {
		case <-ctx.Done():
			cancel()
			wg.Wait()
			break
		default:
			time.Sleep(time.Second * 2)
		}
	}
	for _, e := range m.GetAll() {
		if e.S != nil {
			_ = e.S.ReportError("data side shutting down, task data not completed")
		}
	}
	c.Stop()
	r.Stop()
	return
}
