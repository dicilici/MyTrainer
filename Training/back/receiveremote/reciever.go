package receive

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"train/back/manager"
	"train/back/taskdb"
	"train/pkg"
)

const (
	statusFinished  = "已完成"
	statusFailed    = "已失败"
	statusCancelled = "已取消"
)

type Receiver interface {
	Run() error
	DatabaseRemote(c context.Context, d *DatabaseMsg) (*DatabaseMsgRe, error)
	TrainRemote(c context.Context, t *TrainMsg) (*TrainMsgRe, error)
}

type DefaultReceiver struct {
	file  *os.File
	mux   *sync.RWMutex
	path  string
	db    taskdb.TaskDb
	mgr   manager.Manager
	idm   manager.IdManager
	check func()
	UnimplementedRemoteLogServer
}

func NewDefaultReceiver(file *os.File, mux *sync.RWMutex, p string,
	db taskdb.TaskDb, mgr manager.Manager, idm manager.IdManager, check func()) *DefaultReceiver {
	return &DefaultReceiver{
		file:  file,
		mux:   mux,
		path:  p,
		db:    db,
		mgr:   mgr,
		idm:   idm,
		check: check,
	}
}

func (d *DefaultReceiver) Run() error {
	list, err := net.Listen("tcp", ":50053")
	if err != nil {
		return err
	}
	defer list.Close()
	s := grpc.NewServer()
	RegisterRemoteLogServer(s, d)
	return s.Serve(list)
}

func (d *DefaultReceiver) DatabaseRemote(c context.Context, dm *DatabaseMsg) (*DatabaseMsgRe, error) {
	select {
	case <-c.Done():
		return nil, c.Err()
	default:
	}
	file, _ := os.OpenFile(d.path+dm.TaskId+".txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer file.Close()
	str := fmt.Sprint("received message from external database:", " Id:", dm.TaskId, " Status:", dm.Status, " Msg:", dm.LogMsg)
	pkg.MuxLogWithString(file, str, dm.TaskId, true, d.mux)
	return &DatabaseMsgRe{}, nil
}

func (d *DefaultReceiver) TrainRemote(c context.Context, t *TrainMsg) (*TrainMsgRe, error) {
	select {
	case <-c.Done():
		return nil, c.Err()
	default:
	}
	file, _ := os.OpenFile(d.path+t.TaskId+".txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer file.Close()
	str := fmt.Sprint("received message from external training terminal:", " Id:", t.TaskId, " Status:", t.Status, " Msg:", t.LogMsg)
	pkg.MuxLogWithString(file, str, t.TaskId, true, d.mux)

	if isTerminal(t.Status) {
		d.finalize(t.TaskId, t.Status, t.LogMsg)
	}
	return &TrainMsgRe{}, nil
}

func isTerminal(s string) bool {
	return s == statusFinished || s == statusFailed || s == statusCancelled
}

func (d *DefaultReceiver) finalize(id, status, msg string) {
	v, err := d.mgr.Get(id)
	if err != nil {
		return
	}
	ts := taskdb.TaskStatus{
		C:            v.C,
		Status:       status,
		ErrorMessage: msg,
		StartTime:    v.StartTime,
		EndTime:      time.Now(),
	}
	_ = d.db.Insert(id, ts, time.Now())

	_ = d.mgr.Pop(id)
	if n, ok := extractCounter(id); ok {
		d.idm.InsertId(n)
	}
	if v.S != nil {
		v.S.Disconnect()
	}
	if v.D != nil {
		v.D.Disconnect()
	}
	if d.check != nil {
		d.check()
	}
}

func extractCounter(id string) (int32, bool) {
	f := strings.Fields(id)
	if len(f) == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(f[len(f)-1])
	if err != nil {
		return 0, false
	}
	return int32(n), true
}
