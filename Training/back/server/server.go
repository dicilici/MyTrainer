package cmdClient

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"train/back/Database"
	"train/back/commandLine"
	"train/back/manager"
	"train/back/sender"
	"train/back/taskdb"
	"train/config"
	"train/pkg"
)

type Single struct {
	Single1 *atomic.Bool
	Single2 *atomic.Bool
	mux     *sync.RWMutex
	Closed  *atomic.Bool
}

func NewSingle(s1 *atomic.Bool, s2 *atomic.Bool) *Single {
	var IsClosed atomic.Bool
	IsClosed.Store(false)
	return &Single{
		Single1: s1,
		Single2: s2,
		mux:     &sync.RWMutex{},
		Closed:  &IsClosed,
	}
}

func (s *Single) SetSingle1(status bool, se *grpc.Server) {
	s.Single1.Store(status)
	if s.Single1.Load() == false && s.Single2.Load() == false && s.Closed.Load() == false {
		se.GracefulStop()
		s.Closed.Store(true)
	}
}

func (s *Single) Check(m manager.Manager, se *grpc.Server) {
	if m.GetNumber() == 0 {
		s.Single2.Store(false)
	}
	if s.Single1.Load() == false && s.Single2.Load() == false && s.Closed.Load() == false {
		se.GracefulStop()
		s.Closed.Store(true)
	}
}

type CmdServer interface {
	Run() error
	GetLogPath() string
}

type DefaultCmdServer struct {
	idManager    manager.IdManager
	manager      manager.Manager
	command      commandLine.CommandLogger
	TaskDb       taskdb.TaskDb
	LogPath      string
	CmdSingle    chan int
	TaskSingle   chan int
	SingleDevice *Single
	Server       *grpc.Server
	file         *os.File
	mux          *sync.RWMutex
	UnimplementedManagerLinkServer
}

func NewDefaultCmdServer(mux *sync.RWMutex, s *Single, logPath string, command commandLine.CommandLogger, idManager manager.IdManager, manager manager.Manager, taskdb taskdb.TaskDb, file *os.File) *DefaultCmdServer {
	return &DefaultCmdServer{
		idManager:    idManager,
		manager:      manager,
		command:      command,
		TaskDb:       taskdb,
		LogPath:      logPath,
		CmdSingle:    make(chan int, 1),
		TaskSingle:   make(chan int, 1),
		SingleDevice: s,
		file:         file,
		mux:          mux,
	}
}

func (d *DefaultCmdServer) Run() error {
	list, err := net.Listen("tcp", ":50051")
	if err != nil {
		pkg.MuxLog(d.file, err, strconv.Itoa(-2), false, d.mux)
		return err
	}
	defer list.Close()
	s := grpc.NewServer()
	d.Server = s
	RegisterManagerLinkServer(s, d)
	if err = s.Serve(list); err != nil {
		pkg.MuxLog(d.file, err, strconv.Itoa(-2), false, d.mux)
		return err
	}
	return nil
}

func (d *DefaultCmdServer) GetLogPath() string {
	return d.LogPath
}

func (d *DefaultCmdServer) CheckManager(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	d.SingleDevice.SetSingle1(true, d.Server)
	return &emptypb.Empty{}, nil
}

func (d *DefaultCmdServer) ApplyManager(ctx context.Context, m *ApplyMessage) (*ApplyResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	c := config.NewDefaultConfig()
	err := config.NewDefaultIniter().Init(m.Path, c)
	if err != nil {
		return &ApplyResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	id := d.idManager.GetId()
	dc, err := sender.NewDefaultClient(d.mux, id, ctx, d.LogPath, *c)
	if err != nil {
		return &ApplyResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	db, err := Database.NewDefaultDatabaseHandler(d.mux, id, d.LogPath, *c, ctx)
	if err != nil {
		return &ApplyResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	err = d.manager.Insert(id, *c, dc, db)
	if err != nil {
		return &ApplyResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	args, err := d.manager.Get(id)
	if err != nil {
		return &ApplyResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	re := d.command.ExecuteByName(m.Name, args[1:])
	if re.Error != nil {
		return &ApplyResponse{
			IsOK:     false,
			ErrorMsg: re.Error.Error(),
		}, err
	}
	d.SingleDevice.Check(d.manager, d.Server)
	return &ApplyResponse{
		IsOK:     true,
		ErrorMsg: "",
	}, nil
}

func (d *DefaultCmdServer) TaskManager(ctx context.Context, m *TaskMessage) (*TaskResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var re commandLine.ReturnMsg
	if m.All == true {
		task := d.manager.GetMap()
		for _, v := range task {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			re = d.command.ExecuteByName(m.Name, []interface{}{v[1]})
			if re.Error != nil {
				return &TaskResponse{
					IsOK:     false,
					ErrorMsg: re.Error.Error(),
					Msg:      re.Msg,
				}, re.Error
			}
		}
	} else {
		v, err := d.manager.Get(m.Id)
		if err != nil {
			return &TaskResponse{
				IsOK:     false,
				ErrorMsg: err.Error(),
			}, err
		}
		re = d.command.ExecuteByName(m.Name, []interface{}{v[1]})
		if re.Error != nil {
			return &TaskResponse{
				IsOK:     false,
				ErrorMsg: re.Error.Error(),
				Msg:      "",
			}, err
		}
	}
	return &TaskResponse{
		IsOK:     true,
		ErrorMsg: "",
		Msg:      re.Msg,
	}, nil
}

func (d *DefaultCmdServer) CancelManager(ctx context.Context, m *CancelMessage) (*CancelResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	t, err := d.manager.Get(m.Id)
	if err != nil {
		return &CancelResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	err = d.manager.Pop(m.Id)
	if err != nil {
		return &CancelResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	if len(d.manager.GetMap()) == 0 {
		d.TaskSingle <- 1
	}
	ss := strings.Split(m.Id, "_")
	i, _ := strconv.Atoi(ss[1])
	d.idManager.InsertId(int32(i))
	re := d.command.ExecuteByName(m.Name, []interface{}{t[1], t[2]})
	if re.Error != nil {
		return &CancelResponse{
			IsOK:     false,
			ErrorMsg: re.Error.Error(),
		}, err
	}
	d.SingleDevice.Check(d.manager, d.Server)
	return &CancelResponse{
		IsOK:     true,
		ErrorMsg: "",
	}, nil
}

func (d *DefaultCmdServer) Exit(ctx context.Context, m *ExitMessage) (*ExitResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	d.SingleDevice.SetSingle1(false, d.Server)
	d.SingleDevice.Check(d.manager, d.Server)
	return &ExitResponse{
		IsOK:     true,
		ErrorMsg: "",
	}, nil
}

func (d *DefaultCmdServer) DeleteTaskDb(ctx context.Context, m *DeleteMessage) (*DeleteResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	k := m.Key
	t := m.Time.AsTime()
	err := d.TaskDb.Delete(k, t)
	if err != nil {
		return &DeleteResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
		}, err
	}
	return &DeleteResponse{
		IsOK:     true,
		ErrorMsg: "",
	}, err
}

func (d *DefaultCmdServer) ViewTaskDb(ctx context.Context, m *ViewMessage) (*ViewResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	k := m.Key
	t := m.Time.AsTime()
	err, str := d.TaskDb.View(k, t)
	if err != nil {
		return &ViewResponse{
			IsOK:     false,
			ErrorMsg: err.Error(),
			Msg:      "",
		}, err
	}
	return &ViewResponse{
		IsOK:     true,
		ErrorMsg: "",
		Msg:      str,
	}, err
}

func (d *DefaultCmdServer) CheckNode(ctx context.Context, m *CheckNodeMessage) (*CheckNodeResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var items map[string][]interface{}
	if m.Id == "" {
		items = d.manager.GetMap()
	} else {
		v, err := d.manager.Get(m.Id)
		if err != nil {
			return &CheckNodeResponse{IsOK: false, ErrorMsg: err.Error()}, err
		}
		items = map[string][]interface{}{m.Id: v}
	}

	metrics := make([]*NodeMetrics, 0, len(items)*2)
	for id, v := range items {
		s := v[1].(sender.Client)
		db := v[2].(Database.DatabaseHandler)

		if r, err := s.CheckNode(); err == nil {
			metrics = append(metrics, &NodeMetrics{Id: id, Node: "train", Cpu: r.Cpu, Memory: r.Memory, Disk: r.Disk, DiskIO: r.DiskIO})
		} else {
			pkg.MuxLog(d.file, err, id, false, d.mux)
		}

		if r, err := db.CheckNode(); err == nil {
			metrics = append(metrics, &NodeMetrics{Id: id, Node: "database", Cpu: r.Cpu, Memory: r.Memory, Disk: r.Disk, DiskIO: r.DiskIO})
		} else {
			pkg.MuxLog(d.file, err, id, false, d.mux)
		}
	}

	return &CheckNodeResponse{IsOK: true, Metrics: metrics}, nil
}
