// +build !integration

package main

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type RunTestSuite struct {
	suite.Suite
}

func (s *RunTestSuite) SetupTest() {
	logPrintf = func(format string, v ...interface{}) {}
}

// NewRun

func (s RunTestSuite) Test_NewRun_ReturnsNewStruct() {
	s.NotNil(newRun())
}

// Suite

func TestRunUnitTestSuite(t *testing.T) {
	suite.Run(t, new(RunTestSuite))
}
