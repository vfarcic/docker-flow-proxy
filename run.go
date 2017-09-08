package main

import haproxy "./proxy"

type runner interface {
	Execute(args []string) error
}

type run struct{}

var newRun = func() runner {
	return &run{}
}

// Execute runs the proxy
func (m *run) Execute(args []string) error {
	return haproxy.HaProxy{}.RunCmd([]string{})
}
