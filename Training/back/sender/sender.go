package sender

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"os"
	"strconv"
	"sync"
	"train/config"
	"train/pkg"
)

func NewSendDataset(c config.Dataset) *SendDataset {
	return &SendDataset{
		Input:            c.Input,
		Validation:       int32(c.Validation),
		CategoriesNumber: int32(c.CategoriesNumber),
	}
}

func NewSendTrainConfig(c config.TrainConfig) *SendTrainConfig {
	return &SendTrainConfig{
		Epochs:           int32(c.Epochs),
		LearningRate:     float32(c.LearningRate),
		LossFunction:     c.LossFunction,
		EarlyStop:        c.EarlyStop,
		EarlyStopPatient: int32(c.EarlyStopPatient),
		ModelSave:        c.ModelSave,
		LogSave:          c.LogSave,
		TimeOut:          c.TimeOut,
	}
}

type Client interface {
	GetStorePath() string
	Send() error
	Query() (error, string)
	Cancel() error
	Disconnect() error
}

type DefaultClient struct {
	conn    *grpc.ClientConn
	client  TrainingClient
	ctx     context.Context
	LogPath string
	config  config.Config
	Id      string
	mux     *sync.RWMutex
	wg      *sync.WaitGroup
}

func NewDefaultClient(mux *sync.RWMutex, id string, ctx context.Context, LogPath string, c config.Config) (*DefaultClient, error) {
	conn, err := grpc.NewClient(c.TrainBackendUrl)
	if err != nil {
		return nil, err
	}
	client := NewTrainingClient(conn)
	return &DefaultClient{
		conn:    conn,
		client:  client,
		ctx:     ctx,
		LogPath: LogPath,
		config:  c,
		Id:      id,
		mux:     mux,
		wg:      &sync.WaitGroup{},
	}, nil
}

func (s *DefaultClient) Send() error {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		fileName := fmt.Sprint(s.LogPath, s.Id, ".txt")
		file, err := os.Create(fileName)
		if err != nil {
			return err
		}
		defer file.Close()
		senddataset := NewSendDataset(s.config.Dataset)
		sendtrainconfig := NewSendTrainConfig(s.config.TrainConfig)
		re, err := s.client.SendToTrain(s.ctx, &SendMessage{Dataset: senddataset, TrainConfig: sendtrainconfig, Id: s.Id, RemoteLogURL: os.Getenv("REMOTE_URL")})
		if err != nil {
			pkg.MuxLog(file, err, s.Id, false, s.mux)
			return err
		}
		if re.IsOK == false {
			pkg.MuxLogWithString(file, re.Errors, s.Id, false, s.mux)
			return errors.New(re.Errors)
		}
		return nil
	}
}

func (s *DefaultClient) Query() (error, string) {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err(), s.ctx.Err().Error()
	default:
		fileName := fmt.Sprint(s.LogPath, s.Id, ".txt")
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			pkg.MuxLog(file, err, s.Id, true, s.mux)
			return err, ""
		}
		defer file.Close()
		var str string
		re, err := s.client.QueryTraining(s.ctx, &Query{})
		if err != nil {
			pkg.MuxLog(file, err, s.Id, false, s.mux)
			return err, ""
		} else {
			if !re.IdOK {
				pkg.MuxLogWithString(file, re.Errors, s.Id, false, s.mux)
				return errors.New(re.Errors), ""
			}
			var done string
			if re.Done {
				done = "Done"
			} else {
				done = "No"
			}
			str = fmt.Sprint("check result:" + "Is the training done:" + done + ",Current training epoch:" + strconv.Itoa(int(re.Epoch)) + ",loss:" + fmt.Sprint(re.Loss))
		}
		return nil, str
	}
}

func (s *DefaultClient) Cancel() error {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		fileName := fmt.Sprint(s.LogPath, s.Id, ".txt")
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			pkg.MuxLog(file, err, s.Id, false, s.mux)
			return err
		}
		re, err := s.client.CancelTraining(s.ctx, &Cancel{})
		if err != nil {
			pkg.MuxLog(file, err, s.Id, false, s.mux)
			return err
		} else {
			if !re.IsOK {
				pkg.MuxLogWithString(file, re.Errors, s.Id, false, s.mux)
				return errors.New(re.Errors)
			}
			pkg.MuxLogWithString(file, "Cancel training task", s.Id, false, s.mux)
		}
		return nil
	}
}

func (s *DefaultClient) Disconnect() error {
	s.wg.Wait()
	err := s.conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (s *DefaultClient) GetStorePath() string {
	s.wg.Add(1)
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return ""
	default:
		return s.LogPath
	}
}
