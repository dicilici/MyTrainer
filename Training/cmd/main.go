package main

import (
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"strconv"
	"sync"
	"text/tabwriter"
	"time"
	"train/cmd/cmdClient"
	receive "train/cmd/receiveremote"
	"train/pkg"
)

var conn *grpc.ClientConn
var m cmdClient.ManagerClient
var ctx context.Context
var cancel context.CancelFunc
var r receive.Receiver
var mux *sync.RWMutex
var file *os.File
var LogPath string

func init() {
	backpath := os.Getenv("BACKPATH")
	LogPath = os.Getenv("TRAINCONFIG_PATH")
	file, _ = os.OpenFile(LogPath+"Cmd"+".txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	mux = &sync.RWMutex{}
	ctx, cancel = context.WithCancel(context.Background())
	conn, _ = grpc.NewClient("localhost:50051")
	m = cmdClient.NewDefaultManagerClient(ctx, LogPath, conn, mux, file)
	for i := 1; i <= 5; i++ {
		err := m.Check()
		if err != nil && i != 5 {
			pkg.CallBack(backpath)
			time.Sleep(2 * time.Second)
		} else if err != nil && i == 5 {
			panic("Unable to connect to the manager")
		} else {
			break
		}
	}
	r = receive.NewDefaultReceiver(file, mux, LogPath)
}

func main() {
	go r.Run()
	defer file.Close()
	for {
		select {
		case reErr := <-receive.Ch:
			pkg.Log(file, reErr, strconv.Itoa(-1), true)
			continue
		default:
			cmd, args := pkg.ReadCommand()
			switch cmd {
			case "apply":
				_ = m.Apply(&cmdClient.ApplyMessage{Path: args[0], Name: "apply"})
			case "checkLog":
				id := args[0]
				var s, e time.Time
				var hasS, hasE bool
				if len(args) > 1 && args[1] != "" {
					parsed, err := pkg.HandleTime(args[1])
					if err != nil {
						fmt.Println(err)
						pkg.MuxLog(file, err, strconv.Itoa(-1), false, mux)
						continue
					}
					s, hasS = parsed, true
				}
				if len(args) > 2 && args[2] != "" {
					parsed, err := pkg.HandleTime(args[2])
					if err != nil {
						fmt.Println(err)
						pkg.MuxLog(file, err, strconv.Itoa(-1), false, mux)
						continue
					}
					e, hasE = parsed, true
				}
				if err := pkg.ReadLocalLog(LogPath+id+".txt", s, e, hasS, hasE); err != nil {
					fmt.Println(err)
					pkg.MuxLog(file, err, strconv.Itoa(-1), false, mux)
				}
			case "task":
				b, _ := strconv.ParseBool(args[0])
				_ = m.Task(&cmdClient.TaskMessage{All: b, Id: args[1], Name: "Task"})
			case "cancel":
				b, _ := strconv.ParseBool(args[0])
				_ = m.Cancel(&cmdClient.CancelMessage{All: b, Id: args[1], Name: "cancel"})
			case "exit":
				_ = m.Exit(&cmdClient.ExitMessage{Name: "exit"})
				cancel()
				return
			case "viewtaskdb":
				s, err := pkg.HandleTime(args[1])
				if err != nil {
					fmt.Println(err)
					continue
				}
				ss := timestamppb.New(s)
				_ = m.ViewTaskDb(&cmdClient.ViewMessage{Key: args[0], Time: ss})
			case "deletetaskdb":
				s, err := pkg.HandleTime(args[1])
				if err != nil {
					fmt.Println(err)
					continue
				}
				ss := timestamppb.New(s)
				_ = m.DeleteTaskDb(&cmdClient.DeleteMessage{Key: args[0], Time: ss})
			case "checknode":
				id := ""
				if len(args) > 0 {
					id = args[0]
				}
				resp, err := m.CheckNode(&cmdClient.CheckNodeMessage{Id: id})
				if err != nil {
					fmt.Println(err)
					pkg.MuxLog(file, err, strconv.Itoa(-1), false, mux)
					continue
				}
				if !resp.IsOK {
					fmt.Println(resp.ErrorMsg)
					continue
				}
				printNodeTable(resp.Metrics)
			default:
				fmt.Println("Unknown command")
			}
		}
	}
}

func printNodeTable(metrics []*cmdClient.NodeMetrics) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNODE\tCPU\tMEMORY\tDISK\tDISKIO")
	for _, m := range metrics {
		fmt.Fprintf(w, "%s\t%s\t%.2f%%\t%.2f%%\t%.2f%%\t%.2f%%\n",
			m.Id, m.Node, m.Cpu, m.Memory, m.Disk, m.DiskIO)
	}
	w.Flush()
}
