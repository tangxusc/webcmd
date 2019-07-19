package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/server/cmd"
	"io"
	"strings"
	"time"
)

type EventHandler func(event *cmd.CmdEvent) (*cmd.CmdResult, error)

type Client struct {
	url          string
	eventHandler EventHandler
}

func NewClient(url string, handler EventHandler) *Client {
	url = fmt.Sprintf("ws://%s/events", url)
	return &Client{
		url:          url,
		eventHandler: handler,
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
	_ = conn.Close()
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
				_ = closer.Close()
				panic(e.Error())
			}
			n, e := closer.Write(bytes)
			if e != nil {
				logrus.Errorf("closer.Write error:%v", e.Error())
			}
			logrus.Debugf("消息发送完成,写入字节:%v", n)
			_ = closer.Close()
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

			go func(e *cmd.CmdEvent) {
				result, err := client.eventHandler(e)
				if err != nil {
					result = &cmd.CmdResult{
						Id: e.Id,
					}
					result.Data = []byte(fmt.Sprint(err.Error()))
				}
				if timeOut(e) {
					logrus.Warnf("消息过期,一致性,但不发送,丢弃:%v", e.Id)
					return
				}
				results <- result
			}(event)
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
