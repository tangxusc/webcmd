package conn_manager

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/server/cmd"
	"io"
)

type WebSocketConn struct {
	NodeName string
	Conn     *websocket.Conn
	EventBus chan cmd.Event
}

func (conn *WebSocketConn) Start(ctx context.Context) {
	defer func() {
		e := recover()
		if e != nil {
			logrus.WithField("Nodename", conn.NodeName).Error(e)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			close(conn.EventBus)
			e := conn.Conn.Close()
			if e != nil {
				panic(e.Error())
			}
			return
		case event := <-conn.EventBus:
			closer, e := conn.Conn.NextWriter(websocket.TextMessage)
			if e != nil {
				logrus.Errorf("[%s] Start NextWriter error:%s", conn.NodeName, e)
				return
			}
			n, e := closer.Write(event.Data())
			logrus.Debugf("[%s] 发送消息完成,写入字节:%v", conn.NodeName, n)
			if e != nil {
				logrus.Errorf("[%s] Start Write error:%v", conn.NodeName, e)
				return
			}
			e = closer.Close()
			if e != nil {
				panic(e.Error())
			}
		}
	}
}

func (conn *WebSocketConn) Receive(ReceiveBus chan *cmd.CmdResult, ctx context.Context) {
	defer func() {
		e := recover()
		if e != nil {
			logrus.Errorf("[%s] Receive error:%s", conn.NodeName, e)
		}
	}()
	c := conn.Conn
	for {
		select {
		case <-ctx.Done():
			return
		default:
			messageType, r, err := c.NextReader()
			logrus.Debugf("[%s] 读取到消息,消息类型:%v", conn.NodeName, messageType)
			if err != nil {
				panic(err.Error())
			}
			bytes := make([]byte, 0, 1024*10)
			buf := make([]byte, 1024, 1024)
			for {
				n, err := r.Read(buf)
				if err != nil && err != io.EOF {
					panic(err.Error())
					return
				}
				bytes = append(bytes, buf[:n]...)
				if err == io.EOF {
					break
				}
			}
			logrus.Debugf("[%s] 读取完成:%v", conn.NodeName, len(bytes))
			result := &cmd.CmdResult{}
			err = json.Unmarshal(bytes, result)
			if err != nil {
				logrus.Errorf("[%s] Receive Unmarshal error:%s", conn.NodeName, err.Error())
				return
			}
			ReceiveBus <- result
			logrus.Debugf("[%s] result 放入 ReceiveBus:%s", conn.NodeName, result.Id)
		}
	}
}

func New(nodeName string, origin *websocket.Conn) *WebSocketConn {
	return &WebSocketConn{
		NodeName: nodeName,
		Conn:     origin,
		EventBus: make(chan cmd.Event),
	}
}
