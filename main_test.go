// +build !integration

package main

import (
	"./logging"
	"github.com/stretchr/testify/suite"
	"os"
	"strconv"
	"testing"
	"time"
)

// Setup

type MainTestSuite struct {
	suite.Suite
}

func (s *MainTestSuite) SetupTest() {}

// main

func (s MainTestSuite) Test_Main_InvokesArgsParse() {
	actual := false
	newArgs = func() args {
		actual = true
		return args{}
	}

	main()

	s.True(actual)
}

func (s MainTestSuite) Test_Main_InvokesLogging() {
	defer func() { os.Unsetenv("DEBUG") }()
	tests := []struct {
		debug bool
	}{
		{true},
		{false},
	}
	for _, t := range tests {
		os.Setenv("DEBUG", strconv.FormatBool(t.debug))
		startLoggingOrig := logging.StartLogging
		defer func() { logging.StartLogging = startLoggingOrig }()
		invoked := false
		logging.StartLogging = func() {
			invoked = true
		}

		main()
		time.Sleep(10 * time.Millisecond)

		s.Equal(invoked, t.debug)
	}
}

// Suite

func TestMainUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, new(MainTestSuite))
}
