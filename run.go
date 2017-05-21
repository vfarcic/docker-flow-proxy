package main

import haproxy "./proxy"

type runner interface {
	Execute(args []string) error
}

type Run struct{}

var NewRun = func() runner {
	return &Run{}
}

func (m *Run) Execute(args []string) error {
	return haproxy.HaProxy{}.RunCmd([]string{})
}
