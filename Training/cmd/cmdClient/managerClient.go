package cmdClient

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"os"
	"strconv"
	"sync"
	"train/pkg"
)

type ManagerClient interface {
	Check() error
	Apply(message *ApplyMessage) error
	Task(message *TaskMessage) error
	Cancel(message *CancelMessage) error
	Exit(message *ExitMessage) error
	Disconnect() error
	ViewTaskDb(message *ViewMessage) error
	DeleteTaskDb(message *DeleteMessage) error
	CheckNode(message *CheckNodeMessage) (*CheckNodeResponse, error)
}

type DefaultManagerClient struct {
	Path   string
	conn   *grpc.ClientConn
	client ManagerLinkClient
	ctx    context.Context
	mux    *sync.RWMutex
	file   *os.File
	wg     *sync.WaitGroup
}

func NewDefaultManagerClient(ctx context.Context, path string, conn *grpc.ClientConn, mux *sync.RWMutex, file *os.File) *DefaultManagerClient {
	c := NewManagerLinkClient(conn)
	return &DefaultManagerClient{
		Path:   path,
		conn:   conn,
		client: c,
		ctx:    ctx,
		mux:    mux,
		file:   file,
	}
}

func (c *DefaultManagerClient) Apply(message *ApplyMessage) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		a, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*ApplyResponse, error) {
			return c.client.ApplyManager(c.ctx, message)
		})
		if err != nil {
			pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
			return err
		}
		if a.IsOK == false {
			pkg.MuxLogWithString(c.file, a.ErrorMsg, strconv.Itoa(-1), true, c.mux)
		}
		log.Println("the template has been successfully applied")
		return nil
	}
}

func (c *DefaultManagerClient) Task(message *TaskMessage) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		t, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*TaskResponse, error) {
			return c.client.TaskManager(c.ctx, message)
		})
		if err != nil {
			pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
			return err
		}
		if t.IsOK == false {
			pkg.MuxLogWithString(c.file, t.ErrorMsg, strconv.Itoa(-1), true, c.mux)
		}
		fmt.Println(t.Msg)
		return nil
	}
}

func (c *DefaultManagerClient) Cancel(message *CancelMessage) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		ca, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*CancelResponse, error) {
			return c.client.CancelManager(c.ctx, message)
		})
		if err != nil {
			pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
			return err
		}
		if ca.IsOK == false {
			pkg.MuxLogWithString(c.file, ca.ErrorMsg, strconv.Itoa(-1), true, c.mux)
		}
		fmt.Println("task successfully canceled")
		return nil
	}
}

func (c *DefaultManagerClient) Disconnect() error {
	c.wg.Wait()
	return c.conn.Close()
}

func (c *DefaultManagerClient) CheckNode(message *CheckNodeMessage) (*CheckNodeResponse, error) {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	default:
		return pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*CheckNodeResponse, error) {
			return c.client.CheckNode(c.ctx, message)
		})
	}
}

func (c *DefaultManagerClient) Exit(message *ExitMessage) error {
	e, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*ExitResponse, error) {
		return c.client.Exit(c.ctx, message)
	})
	if err != nil {
		pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
		return err
	}
	if e.IsOK == false {
		pkg.MuxLogWithString(c.file, e.ErrorMsg, strconv.Itoa(-1), true, c.mux)
	}
	err = c.Disconnect()
	if err != nil {
		pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
		return err
	}
	return nil
}

func (c *DefaultManagerClient) Check() error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		_, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*emptypb.Empty, error) {
			return c.client.CheckManager(c.ctx, &emptypb.Empty{})
		})
		if err != nil {
			return err
		}
		return nil
	}
}

func (c *DefaultManagerClient) ViewTaskDb(message *ViewMessage) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		v, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*ViewResponse, error) {
			return c.client.ViewTaskDb(c.ctx, message)
		})
		if err != nil {
			pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
			return err
		}
		if v.IsOK == false {
			pkg.MuxLogWithString(c.file, v.ErrorMsg, strconv.Itoa(-1), true, c.mux)
		}
		fmt.Println(v.Msg)
		return nil
	}
}

func (c *DefaultManagerClient) DeleteTaskDb(message *DeleteMessage) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		d, err := pkg.Retry(c.ctx, pkg.DefaultRetries, pkg.DefaultInterval, func() (*DeleteResponse, error) {
			return c.client.DeleteTaskDb(c.ctx, message)
		})
		if err != nil {
			pkg.MuxLog(c.file, err, strconv.Itoa(-1), true, c.mux)
			return err
		}
		if d.IsOK == false {
			pkg.MuxLogWithString(c.file, d.ErrorMsg, strconv.Itoa(-1), true, c.mux)
		}
		fmt.Println("record deleted successfully")
		return nil
	}
}
