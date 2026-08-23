package pkg

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func GetFlag() (string, string, string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print(">> The log path is not set, please enter the paths below:\n")

	fmt.Print("Please enter the sender log path: ")
	sender, _ := reader.ReadString('\n')
	sender = strings.TrimSpace(sender)
	fmt.Print("\n")

	fmt.Print("Please enter the database log path: ")
	database, _ := reader.ReadString('\n')
	database = strings.TrimSpace(database)
	fmt.Print("\n")

	fmt.Print("Please enter the cmdserver log path: ")
	cmdserver, _ := reader.ReadString('\n')
	cmdserver = strings.TrimSpace(cmdserver)

	return sender, database, cmdserver
}
