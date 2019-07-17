package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const contentType = "text/plain;charset=utf-8"

type Event interface {
	NodeName() string
	Data() []byte
}

type WebSocketConn struct {
	NodeName string
	Conn     *websocket.Conn
	EventBus chan Event
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

type CmdResult struct {
	Id   string `json:"id"`
	Data []byte `json:"data"`
}

func (conn *WebSocketConn) Receive(ReceiveBus chan *CmdResult, ctx context.Context) {
	defer func() {
		e := recover()
		if e != nil {
			logrus.Errorf("[%s] Receive error:%s", conn.NodeName, e)
		}
	}()
	c := conn.Conn
	ints := make(chan int, 10)
	for {
		ints <- 1
		select {
		case <-ctx.Done():
			return
		case <-ints:
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
			result := &CmdResult{}
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
		EventBus: make(chan Event),
	}
}

type ConnManager struct {
	ChanBus    chan Event
	conns      map[string]*WebSocketConn
	subscribes map[string]chan *CmdResult
	ReceiveBus chan *CmdResult
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

func (manager *ConnManager) sendCmd(node string, cmd string, args []string) chan *CmdResult {
	event := NewCmdEvent()
	event.Node = node
	event.Cmd = cmd
	event.Args = args
	event.TimeOut = TimeOutDefault
	add := time.Now().Add(time.Second * time.Duration(event.TimeOut))
	event.EndTime = add

	logrus.Debugf("sendCmd,eventId:%s", event.Id)
	manager.ChanBus <- event
	result := make(chan *CmdResult)
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

const TimeOutDefault int = 10

type CmdEvent struct {
	Id      string    `json:"id"`
	Node    string    `json:"node"`
	Cmd     string    `json:"cmd"`
	Args    []string  `json:"args"`
	TimeOut int       `json:"timeout,omitempty"`
	EndTime time.Time `json:"end_time,omitempty"`
}

func NewCmdEvent() *CmdEvent {
	return &CmdEvent{
		Id: strconv.Itoa(time.Now().Nanosecond()),
	}
}

func (event *CmdEvent) NodeName() string {
	return event.Node
}

func (event *CmdEvent) Data() []byte {
	bytes, e := json.Marshal(event)
	if e != nil {
		panic(e.Error())
	}
	return bytes
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.DebugLevel)
	engine := gin.Default()

	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024 * 100,
		WriteBufferSize: 1024 * 100,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	manager := &ConnManager{
		ChanBus:    make(chan Event),
		conns:      make(map[string]*WebSocketConn),
		subscribes: make(map[string]chan *CmdResult),
		ReceiveBus: make(chan *CmdResult),
	}
	todo, _ := context.WithCancel(context.TODO())
	manager.Start(todo)

	engine.GET("/events", gin.WrapF(func(writer http.ResponseWriter, request *http.Request) {
		conn, e := upgrader.Upgrade(writer, request, nil)
		if e != nil {
			panic(e.Error())
		}

		ctx, cancel := context.WithCancel(todo)
		conn.SetCloseHandler(func(code int, text string) error {
			//TODO:处理closeMessage
			fmt.Println("close", code, text)
			manager.RemoveConn(conn, text, cancel)
			return nil
		})
		logrus.Debugf("新连接:%s", conn.RemoteAddr().String())
		manager.AddConn(conn, conn.RemoteAddr().String(), ctx)
	}))
	engine.GET("/node/:node/cmd/:cmd", func(ctx *gin.Context) {
		nodeString := ctx.Param("node")
		cmdString := ctx.Param("cmd")
		queryString := ctx.Query("args")
		reg := regexp.MustCompile("\\s+")
		args := reg.Split(queryString, -1)

		cmdChan := manager.sendCmd(nodeString, cmdString, args)
		result := <-cmdChan

		if result == nil {
			logrus.Warnf("消息未收到回复:result,nil")
			ctx.Data(http.StatusInternalServerError, contentType, nil)
			return
		}
		ctx.Data(http.StatusOK, contentType, result.Data)
	})
	//性能分析
	go func() {
		http.ListenAndServe(":8081", nil)
	}()
	engine.Run()
}
