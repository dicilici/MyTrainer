package commandLine

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"strings"
	"time"
	"train/back/Database"
	"train/back/noderegistry"
	"train/back/sender"
	"train/back/taskdb"
)

type ReturnMsg struct {
	Msg   string
	Error error
}

type CommandLogger interface {
	Register(name string, value func(args []interface{}) ReturnMsg)
	Init()
	ExecuteByName(name string, args []interface{}) ReturnMsg
}

type DefaultCommandLogger struct {
	List map[string]func(args []interface{}) ReturnMsg
}

func NewDefaultCommandLogger() *DefaultCommandLogger {
	return &DefaultCommandLogger{
		List: make(map[string]func(args []interface{}) ReturnMsg, 10),
	}
}

func (logger *DefaultCommandLogger) Init() {
	logger.Register("apply", apply)
	logger.Register("task", task)
	logger.Register("cancelTask", cancelTask)
	logger.Register("exit", exit)
	logger.Register("deletetaskdb", DeleteTaskDb)
	logger.Register("viewtaskdb", ViewTaskDb)
	logger.Register("checknode", checkNode)
	logger.Register("joinnode", joinNode)
	logger.Register("deletenode", deleteNode)
}

func (logger *DefaultCommandLogger) Register(name string, value func(args []interface{}) ReturnMsg) {
	logger.List[name] = value
}

func (logger *DefaultCommandLogger) ExecuteByName(name string, args []interface{}) ReturnMsg {
	err := logger.List[name](args)
	return err
}

func apply(args []interface{}) ReturnMsg {
	s := args[0].(sender.Client)
	d := args[1].(Database.DatabaseHandler)
	reg := args[2].(noderegistry.Registry)
	trainUrl := args[3].(string)
	dataUrl := args[4].(string)
	err := s.Send()
	if err != nil {
		return ReturnMsg{
			Error: err,
			Msg:   "",
		}
	}
	err = d.Link()
	if err != nil {
		return ReturnMsg{
			Error: err,
			Msg:   "",
		}
	}
	reg.Add(trainUrl, noderegistry.NodeTrain)
	reg.Add(dataUrl, noderegistry.NodeDatabase)
	return ReturnMsg{
		Error: nil,
		Msg:   "",
	}
}

func task(args []interface{}) ReturnMsg {
	s := args[0].(sender.Client)
	err, str := s.Query()
	if err != nil {
		return ReturnMsg{
			Error: err,
			Msg:   "",
		}
	}
	return ReturnMsg{
		Error: nil,
		Msg:   str,
	}
}

func cancelTask(args []interface{}) ReturnMsg {
	s := args[0].(sender.Client)
	err := s.Cancel()
	if err != nil {
		return ReturnMsg{
			Error: err,
			Msg:   "",
		}
	}
	return ReturnMsg{
		Error: nil,
		Msg:   "",
	}
}

func exit(args []interface{}) ReturnMsg {
	return ReturnMsg{
		Error: nil,
		Msg:   "",
	}
}

func ViewTaskDb(args []interface{}) ReturnMsg {
	d := args[0].(taskdb.TaskDb)
	t := args[1].(time.Time)
	k := args[2].(string)
	err, str := d.View(k, t)
	return ReturnMsg{
		Error: err,
		Msg:   str,
	}
}

func DeleteTaskDb(args []interface{}) ReturnMsg {
	d := args[0].(taskdb.TaskDb)
	t := args[1].(time.Time)
	k := args[2].(string)
	err := d.Delete(k, t)
	return ReturnMsg{
		Error: err,
		Msg:   "",
	}
}

func checkNode(args []interface{}) ReturnMsg {
	var sb strings.Builder
	for i := 0; i+2 < len(args); i += 3 {
		url := args[i].(string)
		t := args[i+1].(string)
		id := args[i+2].(string)
		conn, err := grpc.NewClient(url)
		if err != nil {
			fmt.Fprintf(&sb, "%s\t%s\t-1\t-1\t-1\t-1\t%s\n", url, t, id)
			continue
		}
		cpu, mem, disk, io := float32(-1), float32(-1), float32(-1), float32(-1)
		if t == "TRAIN" {
			if r, e := sender.NewTrainingClient(conn).CheckNode(context.Background(), &sender.CheckNodeRequest{}); e == nil {
				cpu, mem, disk, io = r.Cpu, r.Memory, r.Disk, r.DiskIO
			}
		} else {
			if r, e := Database.NewDatabaseLinkClient(conn).CheckNode(context.Background(), &Database.CheckNodeRequest{}); e == nil {
				cpu, mem, disk, io = r.Cpu, r.Memory, r.Disk, r.DiskIO
			}
		}
		conn.Close()
		fmt.Fprintf(&sb, "%s\t%s\t%.2f\t%.2f\t%.2f\t%.2f\t%s\n", url, t, cpu, mem, disk, io, id)
	}
	return ReturnMsg{Msg: sb.String()}
}

func joinNode(args []interface{}) ReturnMsg {
	reg := args[0].(noderegistry.Registry)
	t := noderegistry.NodeType(args[1].(string))
	for i := 2; i < len(args); i++ {
		reg.Add(args[i].(string), t)
	}
	return ReturnMsg{Error: nil, Msg: ""}
}

func deleteNode(args []interface{}) ReturnMsg {
	reg := args[0].(noderegistry.Registry)
	for i := 1; i < len(args); i++ {
		reg.Remove(args[i].(string))
	}
	return ReturnMsg{Error: nil, Msg: ""}
}
