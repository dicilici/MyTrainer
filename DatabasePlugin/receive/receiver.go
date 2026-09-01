package Database

import (
	"context"
	"database/config"
	"database/errortable"
	"database/handler"
	"database/manager"
	"database/pkg"
	"database/report"
	"database/send"
	"errors"
	"google.golang.org/grpc"
	"math"
	"net"
	"os"
	"sync"
)

var TaskCome = make(chan int)

type Receiver interface {
	Run(*sync.WaitGroup) error
	Stop() error
}

type DefaultReceiver struct {
	ctx     context.Context
	Idm     manager.IdManager
	C       config.Config
	Send    send.Sender
	Handle  handler.Handler
	LogPath string
	m       manager.Manager
	et      errortable.ErrorTable
	sever   *grpc.Server
	mux     *sync.RWMutex
	file    *os.File
	UnimplementedDatabaseLinkServer
}

func NewDefaultReceiver(logPath string, m manager.Manager, idm manager.IdManager, ctx context.Context, mux *sync.RWMutex, file *os.File, et errortable.ErrorTable) *DefaultReceiver {
	return &DefaultReceiver{
		ctx:     ctx,
		LogPath: logPath,
		Idm:     idm,
		m:       m,
		et:      et,
		mux:     mux,
		file:    file,
	}
}

func (r *DefaultReceiver) SendToDatabase(ctx context.Context, m *DatabaseConfig) (*Response, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	rep, err := report.NewDefaultReporter(m.RemoteLogURL, context.Background())
	if err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
	}
	sender, err := send.NewDefaultSender(m.TrainURL, m.Id, r.ctx, r.mux, r.file)
	if err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
		if rep != nil {
			rep.Report("receive", err.Error(), m.Id)
		}
		return &Response{
			IsOk:  false,
			Error: "database connection to the training server timed out",
		}, errors.New("database connection to the training server timed out")
	}
	c := config.Config{
		DbName:   m.DBType,
		Account:  m.Account,
		Password: m.Password,
		URL:      m.TrainURL,
	}
	var ss []handler.Select
	for _, se := range m.Criterias {
		ss = append(ss, handler.Select{
			Field:    se.Field,
			Operator: se.Operator,
			Value:    se.Value,
		})
	}
	h, err := handler.NewDefaultHandler(m.Account, m.Password, m.DBType, ss, r.ctx, m.Id, r.mux, r.file)
	if err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
		if rep != nil {
			rep.Report("receive", err.Error(), m.Id)
		}
		return nil, err
	}
	total, err := h.Count()
	if err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
		if rep != nil {
			rep.Report("receive", err.Error(), m.Id)
		}
		return nil, err
	}
	threshold := int64(math.Ceil(0.4 * float64(total)))
	if err := r.et.Insert(m.Id, threshold); err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
		if rep != nil {
			rep.Report("receive", err.Error(), m.Id)
		}
		return nil, err
	}
	taskCtx, taskCancel := context.WithCancel(r.ctx)
	err = r.m.Insert(m.Id, manager.Element{
		C:      c,
		S:      sender,
		H:      h,
		R:      rep,
		Ctx:    taskCtx,
		Cancel: taskCancel,
	})
	if err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
		if rep != nil {
			rep.Report("receive", err.Error(), m.Id)
		}
		return &Response{
			IsOk:  false,
			Error: err.Error(),
		}, err
	}
	err = r.Idm.Insert(m.Id)
	if err != nil {
		pkg.MuxLog(r.file, err, m.Id, false, r.mux)
		if rep != nil {
			rep.Report("receive", err.Error(), m.Id)
		}
		return nil, err
	}
	TaskCome <- 1
	return &Response{
		IsOk:  true,
		Error: "",
	}, nil
}

func (r *DefaultReceiver) Cancel(ctx context.Context, c *CancelMessage) (*CancelResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	e, err := r.m.Get(c.Id)
	if err != nil {
		pkg.MuxLog(r.file, err, c.Id, false, r.mux)
		if e.R != nil {
			e.R.Report("receive", err.Error(), c.Id)
		}
		return &CancelResponse{
			IsOK:  false,
			Error: err.Error(),
		}, err
	}
	err = e.S.Disconnect()
	if err != nil {
		pkg.MuxLog(r.file, err, c.Id, false, r.mux)
		if e.R != nil {
			e.R.Report("receive", err.Error(), c.Id)
		}
		return &CancelResponse{
			IsOK:  false,
			Error: err.Error(),
		}, err
	}
	err = e.H.DisConnect()
	if err != nil {
		pkg.MuxLog(r.file, err, c.Id, false, r.mux)
		if e.R != nil {
			e.R.Report("receive", err.Error(), c.Id)
		}
		return &CancelResponse{
			IsOK:  false,
			Error: err.Error(),
		}, err
	}
	r.m.Pop(c.Id)
	r.Idm.SelectPop(c.Id)
	return &CancelResponse{
		IsOK:  true,
		Error: "",
	}, nil
}

func (r *DefaultReceiver) CheckNode(ctx context.Context, _ *CheckNodeRequest) (*CheckNodeReply, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	cpu, memory, disk, diskIO := pkg.CollectMetrics()
	return &CheckNodeReply{Cpu: cpu, Memory: memory, Disk: disk, DiskIO: diskIO}, nil
}

func (receiver *DefaultReceiver) Run(wg *sync.WaitGroup) error {
	list, err := net.Listen("tcp", "50051")
	if err != nil {
		pkg.MuxLog(receiver.file, err, "-3", false, receiver.mux)
		wg.Done()
		return err
	}
	defer list.Close()
	s := grpc.NewServer()
	receiver.sever = s
	RegisterDatabaseLinkServer(s, receiver)
	if err = s.Serve(list); err != nil {
		pkg.MuxLog(receiver.file, err, "-3", false, receiver.mux)
		wg.Done()
		return err
	}
	wg.Done()
	return nil
}

func (r *DefaultReceiver) Stop() error {
	close(TaskCome)
	r.sever.GracefulStop()
	return nil
}
