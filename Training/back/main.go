package main

import (
	"context"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"train/back/commandLine"
	"train/back/manager"
	cmdClient "train/back/server"
	"train/back/taskdb"
	"train/pkg"
)

var m manager.Manager
var idm manager.IdManager
var s cmdClient.CmdServer
var c commandLine.CommandLogger
var d taskdb.TaskDb
var cp string
var si *cmdClient.Single
var file *os.File
var mux *sync.RWMutex
var ctx context.Context
var cancel context.CancelFunc

func init() {
	ctx, cancel = context.WithCancel(context.Background())
	mux = &sync.RWMutex{}
	var cmdSingle atomic.Bool
	var TaskSingle atomic.Bool
	cmdSingle.Store(true)
	TaskSingle.Store(false)
	si = cmdClient.NewSingle(&cmdSingle, &TaskSingle)
	cp = os.Getenv("TRAINCONFIG_PATH")
	file, _ = os.OpenFile(cp+"Back"+".txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	m = manager.NewDefaultManager()
	idm = manager.NewDefaultIdManager()
	c = commandLine.NewDefaultCommandLogger()
	d = taskdb.NewDefaultTaskDb(cp, ctx)
	s = cmdClient.NewDefaultCmdServer(mux, si, cp, c, idm, m, d, file)
}

func main() {
	defer file.Close()
	err := s.Run()
	if err != nil {
		if err.Error() == "grpc.ErrServerStopped" {
			pkg.MuxLogWithString(file, "cmdserver has been shut down", strconv.Itoa(-2), false, mux)
		} else {
			pkg.MuxLog(file, err, strconv.Itoa(-2), false, mux)
		}
	}
	cancel()
	d.Close()
	m.Close()
}
