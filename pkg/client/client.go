package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/client/cmd"
	"io"
	"os/exec"
	"strings"
	"time"
)

type Client struct {
	url string
}

func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

func (client *Client) Start() {
	ctx, cancel := context.WithCancel(context.TODO())
	defer func() {
		e := recover()
		if e != nil {
			cancel()
			logrus.Warnf("start error:%v", e)
		}
	}()
	logrus.Infof("开始连接到[%s]...", client.url)
	conn, _, e := websocket.DefaultDialer.Dial(client.url, nil)
	if e != nil {
		panic(e.Error())
	}
	logrus.Infof("已连接[%s]...", client.url)

	resultChan := make(chan *cmd.CmdResult)

	go client.sender(ctx, conn, resultChan)
	client.listen(ctx, conn, resultChan)
	conn.Close()
	logrus.Infof("连接断开,开始重连[%s]...", client.url)
}

func (client *Client) sender(ctx context.Context, conn *websocket.Conn, results chan *cmd.CmdResult) {
	for {
		select {
		case <-ctx.Done():
			close(results)
			return
		case entry, ok := <-results:
			if !ok {
				return
			}
			logrus.Debugf("发送消息:%v", entry.Id)
			bytes, e := json.Marshal(entry)
			if e != nil {
				logrus.Errorf("json.Marshal error:%v", e.Error())
				break
			}
			closer, e := conn.NextWriter(websocket.TextMessage)
			if e != nil {
				closer.Close()
				panic(e.Error())
			}
			n, e := closer.Write(bytes)
			if e != nil {
				logrus.Errorf("closer.Write error:%v", e.Error())
			}
			logrus.Debugf("消息发送完成,写入字节:%v", n)
			closer.Close()
		}

	}
}

func (client *Client) listen(ctx context.Context, conn *websocket.Conn, results chan *cmd.CmdResult) {
	ints := make(chan int, 1)
	for {
		ints <- 1
		select {
		case <-ctx.Done():
			//i := conn.Close()
			//if i != nil {
			//	panic(i.Error())
			//}
			return
		case <-ints:
			cmdString, err := readCmd(conn)
			if err != nil {
				return
			}

			event := &cmd.CmdEvent{}
			err = json.Unmarshal([]byte(cmdString), event)
			if err != nil {
				println(err.Error())
				return
			}
			logrus.Debugf("收到消息,解析后:%v", *event)
			if timeOut(event) {
				logrus.Warnf("消息过期,不执行,丢弃:%v", event.Id)
				break
			}

			result, err := execCmd(event)
			if err != nil {
				result.Data = []byte(fmt.Sprint(err.Error()))
			}
			if timeOut(event) {
				logrus.Warnf("消息过期,一致性,但不发送,丢弃:%v", event.Id)
				break
			}
			results <- result
		}
	}
}

func readCmd(conn *websocket.Conn) (string, error) {
	messageType, r, err := conn.NextReader()
	if err != nil {
		return "", err
	}
	logrus.Debugf("收到消息,消息类型:%v", messageType)

	buf := make([]byte, 1024*10)
	builder := &strings.Builder{}
	for {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		builder.Write(buf[0:n])
		if err == io.EOF {
			break
		}
	}
	return builder.String(), nil
}

func timeOut(event *cmd.CmdEvent) bool {
	now := time.Now()
	return now.After(event.EndTime)
}

func execCmd(event *cmd.CmdEvent) (*cmd.CmdResult, error) {
	command := exec.Command(event.Cmd, event.Args...)
	cmdResult := &cmd.CmdResult{
		Id: event.Id,
	}
	closer, e := command.StdoutPipe()
	if e != nil {
		return cmdResult, e
	}
	err := command.Start()
	if err != nil {
		return cmdResult, err
	}
	buf := make([]byte, 1024*10, 1024*10)
	bytes := make([]byte, 0, 1024*10*2)
	for {
		n, err := closer.Read(buf)
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
		return cmdResult, e
	}
	cmdResult.Data = bytes
	return cmdResult, nil
}
