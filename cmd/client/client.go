package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tangxusc/webcmd/pkg/client"
	"github.com/tangxusc/webcmd/pkg/server/cmd"
	"io"
	"os/exec"
	"time"
)

var server string
var debug bool
var retryRange int

var command = cobra.Command{
	Use:   "start",
	Short: "start client",
	Long:  "start client",
	RunE: func(cmd *cobra.Command, args []string) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
			logrus.SetReportCaller(true)
		} else {
			logrus.SetLevel(logrus.WarnLevel)
		}
		for {
			newClient := client.NewClient(server, execCmd)
			newClient.Start()
			time.Sleep(time.Second * time.Duration(retryRange))
		}
	},
}

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{})

	command.PersistentFlags().StringVarP(&server, "server", "s", "127.0.0.1:8080", "server host:port, example: 127.0.0.1:8080")
	command.PersistentFlags().BoolVarP(&debug, "debug", "v", false, "debug mod")
	command.PersistentFlags().IntVarP(&retryRange, "retry time", "r", 5, "retry time")
	_ = command.MarkPersistentFlagRequired("server")
}

func main() {
	e := command.Execute()
	if e != nil {
		println(e.Error())
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
