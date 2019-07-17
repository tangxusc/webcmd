package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/server/conn_manager"
	"net/http"
	_ "net/http/pprof"
	"regexp"
)

const contentType = "text/plain;charset=utf-8"

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.DebugLevel)

	todo, _ := context.WithCancel(context.TODO())
	manager := conn_manager.NewConnManager()
	manager.Start(todo)

	engine := gin.Default()
	engine.GET("/events", gin.WrapF(func(writer http.ResponseWriter, request *http.Request) {
		_, e := manager.Upgrade(writer, request, todo)
		if e != nil {
			panic(e.Error())
		}
	}))
	engine.GET("/node/:node/cmd/:cmd", func(ctx *gin.Context) {
		nodeString := ctx.Param("node")
		cmdString := ctx.Param("cmd")
		queryString := ctx.Query("args")
		reg := regexp.MustCompile("\\s+")
		args := reg.Split(queryString, -1)

		cmdChan := manager.SendCmd(nodeString, cmdString, args)
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
