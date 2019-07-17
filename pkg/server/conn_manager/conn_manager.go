package conn_manager

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/server/cmd"
	"net/http"
	"strings"
	"time"
)

type ConnManager struct {
	ChanBus    chan cmd.Event
	conns      map[string]*WebSocketConn
	subscribes map[string]chan *cmd.CmdResult
	ReceiveBus chan *cmd.CmdResult
	upgrade    *websocket.Upgrader
}

func (manager *ConnManager) AddConn(conn *websocket.Conn, nodeName string, ctx context.Context) {
	split := strings.Split(nodeName, ":")
	nodeName = split[0]
	webConn := New(nodeName, conn)
	manager.conns[nodeName] = webConn
	go webConn.Start(ctx)
	go webConn.Receive(manager.ReceiveBus, ctx)
}

func (manager *ConnManager) RemoveConn(conn *websocket.Conn, nodeName string, cancelFun context.CancelFunc) {
	cancelFun()
	delete(manager.conns, nodeName)
}

func (manager *ConnManager) Start(ctx context.Context) {
	defer func() {
		e := recover()
		if e != nil {
			logrus.Errorf("ConnManager Start error:%s", e)
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(manager.ChanBus)
				logrus.Debugf("ConnManager exit")
				return
			case event := <-manager.ChanBus:
				nodeName := event.NodeName()
				if len(nodeName) > 0 {
					conn, ok := manager.conns[nodeName]
					if ok {
						conn.EventBus <- event
					} else {
						logrus.Warnf("没找到nodeName为:%s 的Conn", nodeName)
					}
				} else {
					//manage
					for _, value := range manager.conns {
						value.EventBus <- event
					}
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(manager.ReceiveBus)
				return
			case result, ok := <-manager.ReceiveBus:
				logrus.Debugf("%v , %v :=<-manager.ReceiveBus", result.Id, ok)
				if !ok {
					return
				}
				targetChan, ok := manager.subscribes[result.Id]
				if ok {
					targetChan <- result
					delete(manager.subscribes, result.Id)
				} else {
					logrus.Warnf("result:%s,没有找到对应的chan", result.Id)
				}
			}
		}
	}()
}

func (manager *ConnManager) SendCmd(node string, cmdString string, args []string) chan *cmd.CmdResult {
	event := cmd.NewCmdEvent()
	event.Node = node
	event.Cmd = cmdString
	event.Args = args
	add := time.Now().Add(time.Second * time.Duration(event.TimeOut))
	event.EndTime = add

	logrus.Debugf("SendCmd,eventId:%s", event.Id)
	manager.ChanBus <- event
	result := make(chan *cmd.CmdResult)
	manager.subscribes[event.Id] = result
	//超时
	go func() {
		duration := time.Second * time.Duration(event.TimeOut)
		after := time.After(duration)
		select {
		case <-after:
			delete(manager.subscribes, event.Id)
			close(result)
		}
	}()
	return result
}

func (manager *ConnManager) Upgrade(writer http.ResponseWriter, request *http.Request, ctx context.Context) (*websocket.Conn, error) {
	conn, e := manager.upgrade.Upgrade(writer, request, nil)
	if e != nil {
		return conn, e
	}
	innerCtx, cancel := context.WithCancel(ctx)
	conn.SetCloseHandler(func(code int, text string) error {
		//TODO:处理closeMessage
		fmt.Println("close", code, text)
		manager.RemoveConn(conn, text, cancel)
		return nil
	})
	logrus.Debugf("新连接:%s", conn.RemoteAddr().String())
	manager.AddConn(conn, conn.RemoteAddr().String(), innerCtx)
	return conn, e
}

func NewConnManager() *ConnManager {
	return &ConnManager{
		ChanBus:    make(chan cmd.Event),
		conns:      make(map[string]*WebSocketConn),
		subscribes: make(map[string]chan *cmd.CmdResult),
		ReceiveBus: make(chan *cmd.CmdResult),
		upgrade: &websocket.Upgrader{
			ReadBufferSize:  1024 * 100,
			WriteBufferSize: 1024 * 100,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}
