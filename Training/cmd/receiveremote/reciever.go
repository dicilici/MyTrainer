package receive

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"os"
	"strconv"
	"sync"
	"train/pkg"
)

var Ch chan error

type Receiver interface {
	Run()
	DatabaseRemote(c context.Context, d *DatabaseMsg) (*DatabaseMsgRe, error)
	TrainRemote(c context.Context, t *TrainMsg) (*TrainMsgRe, error)
}

type DefaultReceiver struct {
	file *os.File
	mux  *sync.RWMutex
	path string
	UnimplementedRemoteLogServer
}

func NewDefaultReceiver(file *os.File, mux *sync.RWMutex, p string) *DefaultReceiver {
	return &DefaultReceiver{
		file: file,
		mux:  mux,
		path: p,
	}
}

func (d *DefaultReceiver) Run() {
	list, err := net.Listen("tcp", ":50053")
	if err != nil {
		pkg.MuxLog(d.file, err, strconv.Itoa(-1), true, d.mux)
		Ch <- err
		return
	}
	defer list.Close()
	s := grpc.NewServer()
	RegisterRemoteLogServer(s, d)
	if err = s.Serve(list); err != nil {
		pkg.MuxLog(d.file, err, strconv.Itoa(-1), true, d.mux)
		Ch <- err
		return
	}
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
	return &TrainMsgRe{}, nil
}
