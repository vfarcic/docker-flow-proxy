// +build !integration

package main

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"net/http"
	"os"
	"testing"
)

type ArgsTestSuite struct {
	suite.Suite
	args args
}

func (s *ArgsTestSuite) SetupTest() {
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return nil
	}
}

// NewArgs

func (s ArgsTestSuite) Test_NewArgs_ReturnsNewStruct() {
	a := newArgs()

	s.IsType(args{}, a)
}

// Parse > Server

func (s ArgsTestSuite) Test_Parse_ParsesServerLongArgs() {
	os.Args = []string{"myProgram", "server"}
	data := []struct {
		expected string
		key      string
		value    *string
	}{
		{"ipFromArgs", "ip", &serverImpl.IP},
		{"portFromArgs", "port", &serverImpl.Port},
	}

	for _, d := range data {
		os.Args = append(os.Args, fmt.Sprintf("--%s", d.key), d.expected)
	}
	args{}.parse()
	for _, d := range data {
		s.Equal(d.expected, *d.value)
	}
}

func (s ArgsTestSuite) Test_Parse_ParsesServerShortArgs() {
	os.Args = []string{"myProgram", "server"}
	data := []struct {
		expected string
		key      string
		value    *string
	}{
		{"ipFromArgs", "i", &serverImpl.IP},
		{"portFromArgs", "p", &serverImpl.Port},
	}

	for _, d := range data {
		os.Args = append(os.Args, fmt.Sprintf("-%s", d.key), d.expected)
	}
	args{}.parse()
	for _, d := range data {
		s.Equal(d.expected, *d.value)
	}
}

func (s ArgsTestSuite) Test_Parse_ServerHasDefaultValues() {
	os.Args = []string{"myProgram", "server"}
	os.Unsetenv("IP")
	os.Unsetenv("PORT")
	data := []struct {
		expected string
		value    *string
	}{
		{"0.0.0.0", &serverImpl.IP},
		{"8080", &serverImpl.Port},
	}

	args{}.parse()
	for _, d := range data {
		s.Equal(d.expected, *d.value)
	}
}

func (s ArgsTestSuite) Test_Parse_ServerDefaultsToEnvVars() {
	os.Args = []string{"myProgram", "server"}
	data := []struct {
		expected string
		key      string
		value    *string
	}{
		{"ipFromEnv", "IP", &serverImpl.IP},
		{"portFromEnv", "PORT", &serverImpl.Port},
	}

	for _, d := range data {
		os.Setenv(d.key, d.expected)
	}
	args{}.parse()
	for _, d := range data {
		s.Equal(d.expected, *d.value)
	}
}

// Suite

func TestArgsUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, new(ArgsTestSuite))
}
