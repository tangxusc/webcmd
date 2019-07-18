package cmd

import "time"

type CmdEvent struct {
	Id      string    `json:"id"`
	Node    string    `json:"node"`
	Cmd     string    `json:"cmd"`
	TimeOut int       `json:"timeout,omitempty"`
	EndTime time.Time `json:"end_time,omitempty"`
}

type CmdResult struct {
	Id   string `json:"id"`
	Data []byte `json:"data"`
}
