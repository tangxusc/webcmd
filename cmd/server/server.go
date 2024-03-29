package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tangxusc/webcmd/pkg/server/conn_manager"
	"net/http"
	_ "net/http/pprof"
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
                    $(".buf").load("/node/127.0.0.1/cmd/?cmd=" + encodeURIComponent(val), function () {
                        $(".cmdInputWarp input").focus();
                        $(".content").append($(".buf").html());
                        //滚动条滚动到底部
                        var scrollHeight = $('html').prop("scrollHeight");
                        $('html').scrollTop(scrollHeight, 200);
                        $(".buf").html("");
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
var debug bool
var port string

var command = cobra.Command{
	Use:   "start",
	Short: "start server",
	Long:  "start server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
			logrus.SetReportCaller(true)
			//性能分析
			go func() {
				http.ListenAndServe(":8081", nil)
			}()
		} else {
			logrus.SetLevel(logrus.WarnLevel)
		}

		todo, _ := context.WithCancel(context.TODO())
		manager := conn_manager.NewConnManager()
		manager.Start(todo)

		engine := gin.Default()
		//engine.LoadHTMLGlob("/home/tangxu/openProject/webcmd/cmd/server/*.html")
		engine.GET("/events", gin.WrapF(func(writer http.ResponseWriter, request *http.Request) {
			_, e := manager.Upgrade(writer, request, todo)
			if e != nil {
				panic(e.Error())
			}
		}))
		engine.GET("/node/:node/cmd/", func(ctx *gin.Context) {
			nodeString := ctx.Param("node")
			query := ctx.Query("cmd")

			cmdChan := manager.SendCmd(nodeString, query)
			result := <-cmdChan

			if result == nil {
				logrus.Warnf("消息未收到回复:result,nil")
				ctx.Data(http.StatusInternalServerError, contentType, nil)
				return
			}
			ctx.Data(http.StatusOK, contentType, result.Data)
		})
		engine.GET("/index", func(ctx *gin.Context) {
			//ctx.HTML(http.StatusOK, "index.html", "")
			ctx.Data(http.StatusOK, "text/html", []byte(indexHtml))
		})

		return engine.Run(fmt.Sprintf(":%s", port))
	},
}

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	command.PersistentFlags().BoolVarP(&debug, "debug", "v", false, "debug model")
	command.PersistentFlags().StringVarP(&port, "port", "p", "8080", "server port")
}

func main() {
	e := command.Execute()
	if e != nil {
		panic(e.Error())
	}
}
