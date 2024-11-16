package sshutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PkgSSHUtilsTestSuite struct {
	suite.Suite
	testSSHPrivateKeyPath string
	cleanupPrivateKey     func()
	mockDialer            *MockSSHDialer
	mockClient            *MockSSHClient
	mockSession           *MockSSHSession
	sshConfig             *SSHConfig
	ctx                   context.Context
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
	s.mockDialer = NewMockSSHDialer()
	s.mockClient = &MockSSHClient{}
	s.mockSession = &MockSSHSession{}

	configInterface, err := NewSSHConfigFunc("example.com", 22, "testuser", s.testSSHPrivateKeyPath)
	s.Require().NoError(err)
	s.sshConfig = configInterface.(*SSHConfig)
	s.sshConfig.SSHDial = s.mockDialer
	s.sshConfig.SetSSHClient(s.mockClient)
}

func (s *PkgSSHUtilsTestSuite) TestNewSSHConfig() {
	assert.Equal(s.T(), "example.com", s.sshConfig.Host)
	assert.Equal(s.T(), 22, s.sshConfig.Port)
	assert.Equal(s.T(), "testuser", s.sshConfig.User)
	assert.NotEmpty(s.T(), s.sshConfig.PrivateKeyMaterial)
	assert.IsType(s.T(), &logger.Logger{}, s.sshConfig.Logger)
	assert.Equal(s.T(), s.mockDialer, s.sshConfig.SSHDial)
}

func (s *PkgSSHUtilsTestSuite) TestConnect() {
	s.mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(s.mockClient, nil)

	client, err := s.sshConfig.Connect()
	s.NoError(err)
	s.NotNil(client)
	s.IsType(&MockSSHClient{}, client)

	s.mockDialer.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestConnectFailure() {
	expectedError := fmt.Errorf("connection error")
	s.mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(nil, expectedError)

	client, err := s.sshConfig.Connect()
	s.Error(err)
	s.Nil(client)
	s.Equal("failed to connect to SSH server: connection error", err.Error())
	s.mockDialer.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestExecuteCommand() {
	s.mockClient.On("NewSession").Return(s.mockSession, nil)
	s.mockClient.On("IsConnected").Return(true)
	expectedOutput := []byte("command output")
	s.mockSession.On("CombinedOutput", "ls -l").Return(expectedOutput, nil)
	s.mockSession.On("Close").Return(nil)

	actualResult, err := s.sshConfig.ExecuteCommand(s.ctx, "ls -l")
	s.NoError(err)
	s.Equal(string(expectedOutput), actualResult)

	s.mockClient.AssertExpectations(s.T())
	s.mockSession.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestExecuteCommandWithRetry() {
	s.mockClient.On("NewSession").Return(s.mockSession, nil)
	s.mockClient.On("IsConnected").Return(true)
	expectedOutput := []byte("command output")
	s.mockSession.On("CombinedOutput", "ls -l").
		Return([]byte{}, fmt.Errorf("temporary error")).Once().
		On("CombinedOutput", "ls -l").Return(expectedOutput, nil).Once()
	s.mockSession.On("Close").Return(nil).Times(2)

	actualResult, err := s.sshConfig.ExecuteCommand(s.ctx, "ls -l")
	s.NoError(err)
	s.Equal(string(expectedOutput), actualResult)

	s.mockClient.AssertExpectations(s.T())
	s.mockSession.AssertNumberOfCalls(s.T(), "CombinedOutput", 2)
	s.mockSession.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestPushFile() {
	s.runPushFileTest(false)
}

func (s *PkgSSHUtilsTestSuite) TestPushFileExecutable() {
	s.runPushFileTest(true)
}

func (s *PkgSSHUtilsTestSuite) runPushFileTest(executable bool) {
	localContent := []byte("test file content")
	if executable {
		localContent = []byte("#!/bin/bash\necho 'Hello, World!'")
	}

	s.mockClient.On("NewSession").Return(s.mockSession, nil)
	s.mockClient.On("IsConnected").Return(true)
	remoteCmd := fmt.Sprintf("rm -f %s && cat > %s", "/remote/path", "/remote/path")
	if executable {
		remoteCmd += fmt.Sprintf(" && chmod -f +x %s", "/remote/path")
	}

	s.mockSession.On("Run", remoteCmd).Return(nil)
	s.mockSession.On("Close").Return(nil)

	err := s.sshConfig.PushFile(s.ctx, "/remote/path", localContent, executable)
	s.NoError(err)

	s.mockClient.AssertExpectations(s.T())
	s.mockSession.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestSystemdServiceOperations() {
	tests := []struct {
		name           string
		operation      interface{}
		serviceName    string
		serviceContent string
		expectedCmd    string
		expectError    bool
	}{
		{
			name:           "InstallSystemdService",
			operation:      s.sshConfig.InstallSystemdService,
			serviceName:    "test_service",
			serviceContent: "service content",
			expectedCmd:    "echo 'service content' | sudo tee /etc/systemd/system/test_service.service > /dev/null",
			expectError:    false,
		},
		{
			name:        "StartService",
			operation:   s.sshConfig.StartService,
			serviceName: "test_service",
			expectedCmd: "sudo systemctl start test_service",
			expectError: false,
		},
		{
			name:        "RestartService",
			operation:   s.sshConfig.RestartService,
			serviceName: "test_service",
			expectedCmd: "sudo systemctl restart test_service",
			expectError: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.mockClient.On("NewSession").Return(s.mockSession, nil)
			s.mockClient.On("IsConnected").Return(true)
			s.mockSession.On("Run", tt.expectedCmd).Return(nil)
			s.mockSession.On("Close").Return(nil)

			var err error
			switch op := tt.operation.(type) {
			case func(context.Context, string, string) error:
				err = op(s.ctx, tt.serviceName, tt.serviceContent)
			case func(context.Context, string) error:
				err = op(s.ctx, tt.serviceName)
			default:
				s.Fail("Unexpected operation type")
			}

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}

			s.mockClient.AssertExpectations(s.T())
			s.mockSession.AssertExpectations(s.T())
		})
	}
}

func TestSSHUtilsSuite(t *testing.T) {
	suite.Run(t, new(PkgSSHUtilsTestSuite))
}
