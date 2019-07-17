package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/server/conn_manager"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strings"
)

const contentType = "text/plain;charset=utf-8"

var indexHtml = `
<!DOCTYPE HTML>
<html>
<head>
    <meta charset="utf-8">
    <title>index</title>
    <style type="text/css">
        html, body {
            background-color: black;
            font-size: 16px;
            font-family: "微软雅黑", serif;
        }

        .cmdInputWarp, .content, .buf {
            color: azure;
        }

        .content {
            padding: 10px 0;
            margin: 0;
        }

        .cmdInputWarp input {
            border: none;
            background: none;
            display: inline-block;
            width: 80%;
            color: azure;
            font-size: 16px;
            font-family: "微软雅黑", serif;
            padding-left: 5px;
            outline: none;
        }
    </style>
    <script src="https://cdn.bootcss.com/jquery/1.12.2/jquery.js"></script>
    <script type="application/javascript">
        $(function () {
            $(".cmdInputWarp input").keypress(function (e) {
                if (e.which === 13) {
                    val = $(".cmdInputWarp input").val();
                    html = $(".cmdInputWarp label").html();
                    $(".content").append(html + " " + val + "</br>");
                    $(".cmdInputWarp input").val('');
                    $(".buf").load("/node/127.0.0.1/cmd/" + encodeURIComponent(val), function () {
						$(".cmdInputWarp input").focus();
                        $(".content").append($(".buf").html());
                        //滚动条滚动到底部
                        var scrollHeight = $('html').prop("scrollHeight");
                        $('html').scrollTop(scrollHeight, 200);
						$(".buf").html("")
                    });
                }
            });
        });
    </script>
</head>
<body>
<div class="buf" style="display: none"></div>
<pre class="content">欢迎登录web-cmd...</br></pre>
<div class="cmdInputWarp">
    <label>webcmd@hostname:</label><input type="text" name="cmd" autofocus/>
</div>
</body>
</html>
`

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.DebugLevel)

	todo, _ := context.WithCancel(context.TODO())
	manager := conn_manager.NewConnManager()
	manager.Start(todo)

	engine := gin.Default()
	engine.LoadHTMLGlob("/home/tangxu/openProject/webcmd/cmd/server/*.html")
	engine.GET("/events", gin.WrapF(func(writer http.ResponseWriter, request *http.Request) {
		_, e := manager.Upgrade(writer, request, todo)
		if e != nil {
			panic(e.Error())
		}
	}))
	engine.GET("/node/:node/cmd/:cmd", func(ctx *gin.Context) {
		nodeString := ctx.Param("node")
		cmdString := ctx.Param("cmd")
		cmdString = strings.TrimSpace(cmdString)
		var args = make([]string, 0)
		reg := regexp.MustCompile("\\s+")
		split := reg.Split(cmdString, -1)
		if len(split) > 1 {
			args = split[1:]
			cmdString = split[0]
		} else {
			args = nil
		}
		cmdChan := manager.SendCmd(nodeString, cmdString, args)
		result := <-cmdChan

		if result == nil {
			logrus.Warnf("消息未收到回复:result,nil")
			ctx.Data(http.StatusInternalServerError, contentType, nil)
			return
		}
		ctx.Data(http.StatusOK, contentType, result.Data)
	})
	engine.GET("/index", func(ctx *gin.Context) {
		ctx.Data(http.StatusOK, "text/html", []byte(indexHtml))
	})
	//性能分析
	go func() {
		http.ListenAndServe(":8081", nil)
	}()
	engine.Run()
}
