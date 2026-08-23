package report

import (
	"context"
	"errors"
	"sync"

	"google.golang.org/grpc"
)

type Reporter interface {
	Report(status string, logMsg string, taskId string) error
	Close() error
}

type DefaultReporter struct {
	conn   *grpc.ClientConn
	client RemoteLogClient
	ctx    context.Context
	wg     *sync.WaitGroup
}

func NewDefaultReporter(url string, ctx context.Context) (Reporter, error) {
	conn, err := grpc.NewClient(url)
	if err != nil {
		return nil, err
	}
	client := NewRemoteLogClient(conn)
	return &DefaultReporter{
		conn:   conn,
		client: client,
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}, nil
}

func (r *DefaultReporter) Report(status string, logMsg string, taskId string) error {
	r.wg.Add(1)
	defer r.wg.Done()
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
		re, err := r.client.DatabaseRemote(r.ctx, &DatabaseMsg{Status: status, LogMsg: logMsg, TaskId: taskId})
		if err != nil {
			return err
		}
		if re.IsOk == false {
			return errors.New(re.ErrorMsg)
		}
		return nil
	}
}

func (r *DefaultReporter) Close() error {
	r.wg.Wait()
	return r.conn.Close()
}
