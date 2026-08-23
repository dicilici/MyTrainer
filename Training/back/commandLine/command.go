package commandLine

import (
	"time"
	"train/back/Database"
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
	d := args[1].(Database.DatabaseHandler)
	err := s.Cancel()
	if err != nil {
		return ReturnMsg{
			Error: err,
			Msg:   "",
		}
	}
	s.Disconnect()
	d.Disconnect()
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
