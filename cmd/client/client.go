package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/tangxusc/webcmd/pkg/client"
	"os"
	"time"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)
	url := os.Args[1]
	url = fmt.Sprintf("ws://%s/events", url)
	for {
		newClient := client.NewClient(url)
		newClient.Start()
		time.Sleep(time.Second * 5)
	}
}
