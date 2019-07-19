package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/client"
	"github.com/tangxusc/webcmd/pkg/server/cmd"
	"io"
	"os"
	"os/exec"
	"time"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetLevel(logrus.DebugLevel)
	//logrus.SetReportCaller(true)
	url := os.Args[1]
	url = fmt.Sprintf("ws://%s/events", url)
	for {
		newClient := client.NewClient(url, execCmd)
		newClient.Start()
		time.Sleep(time.Second * 5)
	}
}

func execCmd(event *cmd.CmdEvent) (*cmd.CmdResult, error) {
	command := exec.Command("/bin/sh", "-c", event.Cmd)
	cmdResult := &cmd.CmdResult{
		Id: event.Id,
	}
	out, e := command.StdoutPipe()
	if e != nil {
		return cmdResult, e
	}
	errOut, e := command.StderrPipe()
	if e != nil {
		return cmdResult, e
	}
	reader := io.MultiReader(errOut, out)
	err := command.Start()
	if err != nil {
		return cmdResult, err
	}
	buf := make([]byte, 1024*10, 1024*10)
	bytes := make([]byte, 0, 1024*10*2)
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return cmdResult, err
		}
		bytes = append(bytes, buf[:n]...)
		if err == io.EOF {
			break
		}
	}
	e = command.Wait()
	if e != nil {
		logrus.Warnf("执行命令出现错误,命令:%v,错误:%v", event.Cmd, e)
	}
	cmdResult.Data = bytes
	return cmdResult, nil
}
