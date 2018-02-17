package proxy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilTestSuite struct {
	suite.Suite
}

func (s *UtilTestSuite) SetupTest() {}

func TestUtilUnitTestSuite(t *testing.T) {
	os.Setenv("SEPARATOR", ",")
	suite.Run(t, new(UtilTestSuite))
}

// cmdRunHa

func (s *UtilTestSuite) Test_HaProxyCmd_DoesNotReturnErrorWhenStdErrIsEmpty() {
	haProxyCmdOrig := haProxyCmd
	defer func() { haProxyCmd = haProxyCmdOrig }()
	haProxyCmd = "ls"
	err := cmdRunHa([]string{"-l"})

	s.NoError(err)
}

func (s *UtilTestSuite) Test_HaProxyCmd_ReturnsError_WhenStdErrIsNotEmpty() {
	haProxyCmdOrig := haProxyCmd
	defer func() { haProxyCmd = haProxyCmdOrig }()
	haProxyCmd = "ls"
	err := cmdRunHa([]string{"-j"}) // `-j` argument is invalid

	s.Error(err)
}
