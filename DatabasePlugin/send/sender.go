package send

import (
	"context"
	"database/pkg"
	"errors"
	"google.golang.org/grpc"
	"os"
	"sync"
	"time"
)

const taskStoppedMsg = "no matching training task"

var ErrTaskStopped = errors.New("task stopped")

type Sender interface {
	SendToTrain(t *ToTrain) error
	CheckStatus() error
	Finish() error
	ReportError(errMsg string) error
	Disconnect() error
}

type DefaultSender struct {
	Conn         *grpc.ClientConn
	Client       WithTrainClient
	TrainAddress string
	Id           string
	ctx          context.Context
	wg           *sync.WaitGroup
	mux          *sync.RWMutex
	file         *os.File
}

func NewDefaultSender(trainAddress string, id string, ctx context.Context, mux *sync.RWMutex, file *os.File) (*DefaultSender, error) {
	var conn *grpc.ClientConn
	var err error
	for i := 1; i <= 600; i++ {
		conn, err = grpc.NewClient(trainAddress)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	client := NewWithTrainClient(conn)
	return &DefaultSender{
		Conn:         conn,
		Client:       client,
		TrainAddress: trainAddress,
		Id:           id,
		ctx:          ctx,
		wg:           &sync.WaitGroup{},
		mux:          mux,
		file:         file,
	}, nil
}

func (s *DefaultSender) CheckStatus() error {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		re, err := pkg.Retry(s.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*CheckResponse, error) {
			return s.Client.Check(s.ctx, &CheckStatus{})
		})
		if err != nil {
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		if re.IsOK == false {
			err := errors.New(re.Msg)
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		return nil
	}
}

func (s *DefaultSender) SendToTrain(t *ToTrain) error {
	t.Id = s.Id
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		re, err := pkg.Retry(s.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*ToTrainResponse, error) {
			return s.Client.SendTrain(s.ctx, t)
		})
		if err != nil {
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		if re.IsOK == false {
			if re.Msg == taskStoppedMsg {
				return ErrTaskStopped
			}
			err := errors.New(re.Msg)
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		return nil
	}
}

func (s *DefaultSender) Finish() error {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		re, err := pkg.Retry(s.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*ToTrainResponse, error) {
			return s.Client.Finish(s.ctx, &FinishMessage{Id: s.Id})
		})
		if err != nil {
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		if re.IsOK == false {
			if re.Msg == taskStoppedMsg {
				return ErrTaskStopped
			}
			err := errors.New(re.Msg)
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		return nil
	}
}

func (s *DefaultSender) ReportError(errMsg string) error {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		re, err := pkg.Retry(s.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*ToTrainResponse, error) {
			return s.Client.ReportError(s.ctx, &ErrorMessage{Id: s.Id, Error: errMsg})
		})
		if err != nil {
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		if re.IsOK == false {
			err := errors.New(re.Msg)
			pkg.MuxLog(s.file, err, s.Id, false, s.mux)
			return err
		}
		return nil
	}
}

func (s *DefaultSender) Disconnect() error {
	s.wg.Wait()
	return s.Conn.Close()
}
