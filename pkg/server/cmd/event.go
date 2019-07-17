package cmd

type Event interface {
	NodeName() string
	Data() []byte
}
