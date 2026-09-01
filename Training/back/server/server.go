package cmdClient

import (
	"context"
	"fmt"
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
	"train/back/noderegistry"
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
	if m.GetNumber() > 0 {
		s.Single2.Store(true)
	} else {
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
	CheckSingle()
}

type DefaultCmdServer struct {
	idManager    manager.IdManager
	manager      manager.Manager
	command      commandLine.CommandLogger
	TaskDb       taskdb.TaskDb
	Registry     noderegistry.Registry
	LogPath      string
	CmdSingle    chan int
	TaskSingle   chan int
	SingleDevice *Single
	Server       *grpc.Server
	file         *os.File
	mux          *sync.RWMutex
	UnimplementedManagerLinkServer
}

func NewDefaultCmdServer(mux *sync.RWMutex, s *Single, logPath string, command commandLine.CommandLogger, idManager manager.IdManager, manager manager.Manager, taskdb taskdb.TaskDb, registry noderegistry.Registry, file *os.File) *DefaultCmdServer {
	return &DefaultCmdServer{
		idManager:    idManager,
		manager:      manager,
		command:      command,
		TaskDb:       taskdb,
		Registry:     registry,
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

func (d *DefaultCmdServer) CheckSingle() {
	d.SingleDevice.Check(d.manager, d.Server)
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
	re := d.command.ExecuteByName(m.Name, []interface{}{args.S, args.D, d.Registry, c.TrainBackendUrl, fmt.Sprint(c.Db.Address, ":", c.Db.Port)})
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
			re = d.command.ExecuteByName(m.Name, []interface{}{v.S})
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
		re = d.command.ExecuteByName(m.Name, []interface{}{v.S})
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
	re := d.command.ExecuteByName(m.Name, []interface{}{t.S, t.D})
	if re.Error != nil {
		return &CancelResponse{
			IsOK:     false,
			ErrorMsg: re.Error.Error(),
		}, err
	}
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

func (d *DefaultCmdServer) JoinNode(ctx context.Context, m *JoinNodeMessage) (*JoinNodeResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	args := []interface{}{d.Registry, m.NodeType}
	for _, u := range m.Urls {
		args = append(args, u)
	}
	re := d.command.ExecuteByName("joinnode", args)
	if re.Error != nil {
		return &JoinNodeResponse{IsOK: false, ErrorMsg: re.Error.Error()}, re.Error
	}
	return &JoinNodeResponse{IsOK: true, ErrorMsg: ""}, nil
}

func (d *DefaultCmdServer) DeleteNode(ctx context.Context, m *DeleteNodeMessage) (*DeleteNodeResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	args := []interface{}{d.Registry}
	for _, u := range m.Urls {
		args = append(args, u)
	}
	re := d.command.ExecuteByName("deletenode", args)
	if re.Error != nil {
		return &DeleteNodeResponse{IsOK: false, ErrorMsg: re.Error.Error()}, re.Error
	}
	return &DeleteNodeResponse{IsOK: true, ErrorMsg: ""}, nil
}

func (d *DefaultCmdServer) CheckNode(ctx context.Context, m *CheckNodeMessage) (*CheckNodeResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var args []interface{}
	if m.Id != "" {
		v, err := d.manager.Get(m.Id)
		if err != nil {
			return &CheckNodeResponse{IsOK: false, ErrorMsg: err.Error()}, err
		}
		trainUrl := v.C.TrainBackendUrl
		dataUrl := fmt.Sprint(v.C.Db.Address, ":", v.C.Db.Port)
		args = []interface{}{trainUrl, string(noderegistry.NodeTrain), m.Id, dataUrl, string(noderegistry.NodeDatabase), m.Id}
	} else {
		trainIds := make(map[string][]string)
		dataIds := make(map[string][]string)
		for id, v := range d.manager.GetMap() {
			trainIds[v.C.TrainBackendUrl] = append(trainIds[v.C.TrainBackendUrl], id)
			dataUrl := fmt.Sprint(v.C.Db.Address, ":", v.C.Db.Port)
			dataIds[dataUrl] = append(dataIds[dataUrl], id)
		}
		for _, n := range d.Registry.GetAll() {
			var ids []string
			if n.Type == noderegistry.NodeTrain {
				ids = trainIds[n.Url]
			} else {
				ids = dataIds[n.Url]
			}
			args = append(args, n.Url, string(n.Type), strings.Join(ids, ","))
		}
	}
	re := d.command.ExecuteByName("checknode", args)
	if re.Error != nil {
		return &CheckNodeResponse{IsOK: false, ErrorMsg: re.Error.Error()}, re.Error
	}
	return &CheckNodeResponse{IsOK: true, Metrics: parseNodeMetrics(re.Msg)}, nil
}

func parseNodeMetrics(s string) []*NodeMetrics {
	var metrics []*NodeMetrics
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			continue
		}
		cpu, _ := strconv.ParseFloat(f[2], 32)
		mem, _ := strconv.ParseFloat(f[3], 32)
		disk, _ := strconv.ParseFloat(f[4], 32)
		io, _ := strconv.ParseFloat(f[5], 32)
		metrics = append(metrics, &NodeMetrics{Url: f[0], Node: f[1], Cpu: float32(cpu), Memory: float32(mem), Disk: float32(disk), DiskIO: float32(io), Id: f[6]})
	}
	return metrics
}
