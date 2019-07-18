package cmd

import (
	"encoding/json"
	"strconv"
	"time"
)

type CmdResult struct {
	Id   string `json:"id"`
	Data []byte `json:"data"`
}

const TimeOutDefault int = 10

type CmdEvent struct {
	Id      string    `json:"id"`
	Node    string    `json:"node"`
	Cmd     string    `json:"cmd"`
	TimeOut int       `json:"timeout,omitempty"`
	EndTime time.Time `json:"end_time,omitempty"`
}

func NewCmdEvent() *CmdEvent {
	return &CmdEvent{
		Id:      strconv.Itoa(time.Now().Nanosecond()),
		TimeOut: TimeOutDefault,
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
