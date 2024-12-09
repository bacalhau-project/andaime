package sshutils

import (
	"context"
	"io"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	ssh_mock "github.com/bacalhau-project/andaime/mocks/sshutils"
	sshutils_interfaces "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
)

type PkgSSHUtilsTestSuite struct {
	suite.Suite
	testSSHPrivateKeyPath string
	cleanupPrivateKey     func()
	ctx                   context.Context
	sshConfig             sshutils_interfaces.SSHConfiger
}

func (s *PkgSSHUtilsTestSuite) SetupSuite() {
	_, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	s.testSSHPrivateKeyPath = testSSHPrivateKeyPath
	s.cleanupPrivateKey = cleanupPrivateKey
	s.T().Cleanup(func() {
		cleanupPublicKey()
		s.cleanupPrivateKey()
	})
}

func (s *PkgSSHUtilsTestSuite) SetupTest() {
	s.ctx = context.Background()
	sshConfig, err := NewSSHConfig("test-host", 22, "test-user", s.testSSHPrivateKeyPath)
	s.Require().NoError(err)
	s.sshConfig = sshConfig
}

func (s *PkgSSHUtilsTestSuite) TestExecuteCommand() {
	expectedOutput := "command output"

	// Create mock session
	mockSession := &ssh_mock.MockSSHSessioner{}
	mockSession.On("Run", "ls -l").Return(nil)
	mockSession.On("Close").Return(nil)
	mockSession.On("SetStdout", mock.MatchedBy(func(w interface{}) bool {
		writer := w.(io.Writer)
		_, err := writer.Write([]byte(expectedOutput))
		return err == nil
	})).Return()
	mockSession.On("SetStderr", mock.Anything).Return()

	// Create mock SSH client
	mockClient := &ssh_mock.MockSSHClienter{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockClient.On("Close").Return(nil)
	mockClient.On("GetClient").Return(&ssh.Client{}).Maybe()
	mockClient.On("Connect").Return(mockClient, nil).Maybe()
	mockClient.On("IsConnected").Return(true).Maybe()

	// Create mock client creator
	mockClientCreator := &ssh_mock.MockSSHClientCreator{}
	mockClientCreator.On("NewClient",
		"test-host",
		22,
		"test-user",
		s.testSSHPrivateKeyPath,
		mock.Anything,
	).Return(mockClient, nil)

	// Set mock client creator
	s.sshConfig.SetSSHClientCreator(mockClientCreator)

	// Execute command
	actualOutput, err := s.sshConfig.ExecuteCommand(s.ctx, "ls -l")
	s.NoError(err)
	s.Equal(expectedOutput, actualOutput)

	// Verify expectations
	mockSession.AssertExpectations(s.T())
	mockClient.AssertExpectations(s.T())
	mockClientCreator.AssertExpectations(s.T())
}

func TestSSHUtilsSuite(t *testing.T) {
	suite.Run(t, new(PkgSSHUtilsTestSuite))
}
