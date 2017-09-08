package main

import (
	"github.com/jessevdk/go-flags"
	"os"
)

type args struct{}

var newArgs = func() args {
	return args{}
}

func (a args) parse() error {
	parser := flags.NewParser(nil, flags.Default)
	parser.AddCommand("server", "Runs the server", "Runs the server", &serverImpl)
	parser.ParseArgs(os.Args[1:])
	return nil
}
