package Database

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"os"
	"sync"
	"train/back/selector"
	"train/config"
	"train/pkg"
)

func NewCancelMessage() *CancelMessage {
	return &CancelMessage{}
}

func NewDatabaseConfig(c config.Config) *DatabaseConfig {
	criterias := make([]*DataCriteria, 0, len(c.Db.Selector.(*selector.DefaultSelector).Conditions))
	for _, condition := range c.Db.Selector.(*selector.DefaultSelector).Conditions {
		criterias = append(criterias, &DataCriteria{
			Field:    condition.Field,
			Value:    condition.Value,
			Operator: condition.Operator,
		})
	}
	return &DatabaseConfig{
		Account:      c.Db.Account,
		Password:     c.Db.Password,
		Criterias:    criterias,
		TrainURL:     c.TrainDataUrl,
		DBType:       c.Db.DbName,
		RemoteLogURL: os.Getenv("REMOTE_URL"),
	}
}

type DatabaseHandler interface {
	Link() error
	Cancel() error
	CheckNode() (*CheckNodeReply, error)
	Disconnect() error
}

type DefaultDatabaseHandler struct {
	conn    *grpc.ClientConn
	client  DatabaseLinkClient
	Id      string
	LogPath string
	ctx     context.Context
	c       config.Config
	mux     *sync.RWMutex
	wg      *sync.WaitGroup
}

func NewDefaultDatabaseHandler(mux *sync.RWMutex, Id string, LogPath string, c config.Config, ctx context.Context) (*DefaultDatabaseHandler, error) {
	url := fmt.Sprint(c.Db.Address, ":", c.Db.Port)
	conn, err := grpc.NewClient(url)
	if err != nil {
		return nil, err
	}
	client := NewDatabaseLinkClient(conn)
	return &DefaultDatabaseHandler{
		conn:    conn,
		client:  client,
		Id:      Id,
		LogPath: LogPath,
		ctx:     ctx,
		c:       c,
		mux:     mux,
		wg:      &sync.WaitGroup{},
	}, nil
}

func (h *DefaultDatabaseHandler) Link() error {
	h.wg.Add(1)
	defer h.wg.Done()
	select {
	case <-h.ctx.Done():
		return h.ctx.Err()
	default:
		filename := fmt.Sprint(h.LogPath, h.Id, ".txt")
		file, err := os.Create(filename)
		defer file.Close()
		if err != nil {
			pkg.MuxLog(file, err, h.Id, false, h.mux)
			return err
		}
		d := NewDatabaseConfig(h.c)
		d.Id = h.Id
		re, err := pkg.Retry(h.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*Response, error) {
			return h.client.SendToDatabase(h.ctx, d)
		})
		if err != nil {
			pkg.MuxLog(file, err, h.Id, false, h.mux)
			return err
		}
		if re.IsOk == false {
			pkg.MuxLog(file, errors.New(re.Error), h.Id, false, h.mux)
			return errors.New(re.Error)
		} else {
			pkg.MuxLogWithString(file, "The database has been connected", h.Id, false, h.mux)
		}
		return nil
	}
}

func (h *DefaultDatabaseHandler) Cancel() error {
	h.wg.Add(1)
	defer h.wg.Done()
	select {
	case <-h.ctx.Done():
		return h.ctx.Err()
	default:
		filename := fmt.Sprint(h.LogPath, h.Id, ".txt")
		file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			pkg.MuxLog(file, err, h.Id, false, h.mux)
			return err
		}
		c := &CancelMessage{
			Id: h.Id,
		}
		re, err := pkg.Retry(h.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*CancelResponse, error) {
			return h.client.Cancel(h.ctx, c)
		})
		if err != nil {
			pkg.MuxLog(file, err, h.Id, false, h.mux)
			return err
		}
		if re.IsOK == false {
			pkg.MuxLog(file, errors.New(re.Error), h.Id, false, h.mux)
			return errors.New(re.Error)
		} else {
			pkg.MuxLogWithString(file, "The database connection has been cancelled", h.Id, false, h.mux)
		}
		return nil
	}
}

func (h *DefaultDatabaseHandler) CheckNode() (*CheckNodeReply, error) {
	h.wg.Add(1)
	defer h.wg.Done()
	select {
	case <-h.ctx.Done():
		return nil, h.ctx.Err()
	default:
		return pkg.Retry(h.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*CheckNodeReply, error) {
			return h.client.CheckNode(h.ctx, &CheckNodeRequest{})
		})
	}
}

func (h *DefaultDatabaseHandler) Disconnect() error {
	h.wg.Wait()
	err := h.conn.Close()
	if err != nil {
		return err
	}
	return nil
}
