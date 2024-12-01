package sshutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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
	sshClient             *ssh_mock.MockSSHClienter
	sshSession            *ssh_mock.MockSSHSessioner
	sshConfig             sshutils_interfaces.SSHConfiger
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
	s.sshClient = &ssh_mock.MockSSHClienter{}
	s.sshSession = &ssh_mock.MockSSHSessioner{}
	sshConfig, err := NewSSHConfigFunc(
		"example.com",
		22, //nolint:mnd
		"testuser",
		s.testSSHPrivateKeyPath,
	)
	s.Require().NoError(err)
	s.sshConfig = sshConfig
	s.sshConfig.SetSSHClienter(s.sshClient)

	// Mock the validateSSHConnection method to bypass network checks
	s.sshConfig.SetValidateSSHConnection(func() error {
		return nil
	})

	// Add default mock expectations for common methods
	s.sshClient.On("IsConnected").Return(true)
	s.sshClient.On("NewSession").Return(s.sshSession, nil)
	s.sshSession.On("Close").Return(nil)
}

func (s *PkgSSHUtilsTestSuite) TestConnect() {
	mockSSHClient := &ssh.Client{}
	s.sshClient.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(&SSHClientWrapper{Client: mockSSHClient}, nil)

	client, err := s.sshConfig.Connect()
	s.NoError(err)
	s.NotNil(client)
	s.IsType(&SSHClientWrapper{}, client)

	s.sshClient.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestConnectFailure() {
	expectedError := fmt.Errorf("connection error")
	s.sshClient.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(nil, expectedError)

	client, err := s.sshConfig.Connect()
	s.Error(err)
	s.Nil(client)
	s.Contains(err.Error(), "failed to dial")
	s.sshClient.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestExecuteCommand() {
	s.sshClient.On("NewSession").Return(s.sshSession, nil)
	s.sshClient.On("IsConnected").Return(true)
	expectedOutput := []byte("command output")
	s.sshSession.On("CombinedOutput", "ls -l").Return(expectedOutput, nil)
	s.sshSession.On("Close").Return(nil)

	actualResult, err := s.sshConfig.ExecuteCommand(s.ctx, "ls -l")
	s.NoError(err)
	s.Equal(string(expectedOutput), actualResult)

	s.sshClient.AssertExpectations(s.T())
	s.sshSession.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestExecuteCommandWithRetry() {
	s.sshClient.On("NewSession").Return(s.sshSession, nil).Times(2)
	s.sshClient.On("IsConnected").Return(true)
	expectedOutput := []byte("command output")
	s.sshSession.On("CombinedOutput", "ls -l").
		Return([]byte{}, fmt.Errorf("temporary error")).Once().
		On("CombinedOutput", "ls -l").Return(expectedOutput, nil).Once()
	s.sshSession.On("Close").Return(nil).Times(2)

	actualResult, err := s.sshConfig.ExecuteCommand(s.ctx, "ls -l")
	s.NoError(err)
	s.Equal(string(expectedOutput), actualResult)

	s.sshClient.AssertExpectations(s.T())
	s.sshSession.AssertNumberOfCalls(s.T(), "CombinedOutput", 2)
	s.sshSession.AssertExpectations(s.T())
}

type mockWriteCloser struct {
	*bytes.Buffer
	closeFunc func() error
}

func (m *mockWriteCloser) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockSFTPClient struct {
	mock.Mock
}

// Verify that mockSFTPClient implements SFTPClienter
var _ sshutils_interfaces.SFTPClienter = &mockSFTPClient{}

func (m *mockSFTPClient) Create(path string) (io.WriteCloser, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (m *mockSFTPClient) Open(path string) (io.ReadCloser, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *mockSFTPClient) MkdirAll(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *mockSFTPClient) Chmod(path string, mode os.FileMode) error {
	args := m.Called(path, mode)
	return args.Error(0)
}

func (m *mockSFTPClient) Close() error {
	args := m.Called()
	return args.Error(0)
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

	// Create mock write closer
	mockFile := &mockWriteCloser{
		Buffer: bytes.NewBuffer(nil),
		closeFunc: func() error {
			return nil
		},
	}

	// Create mock SFTP client
	mockSFTP := &mockSFTPClient{}
	mockSFTP.On("Create", "/remote/path").Return(mockFile, nil)
	if executable {
		mockSFTP.On("Chmod", "/remote/path", os.FileMode(0755)).Return(nil)
	}
	mockSFTP.On("Close").Return(nil)

	// Save the original creator and restore it after the test
	originalCreator := DefaultSFTPClientCreator
	defer func() { DefaultSFTPClientCreator = originalCreator }()

	// Set up our test creator that returns the mock
	var testCreator sshutils_interfaces.SFTPClientCreator = func(client *ssh.Client) (sshutils_interfaces.SFTPClienter, error) {
		return mockSFTP, nil
	}
	DefaultSFTPClientCreator = testCreator

	// Mock GetClient to return a mock ssh.Client
	mockSSHClient := &ssh.Client{}
	s.sshClient.On("GetClient").Return(mockSSHClient)

	err := s.sshConfig.PushFile(s.ctx, "/remote/path", localContent, executable)
	s.NoError(err)

	// Verify the content was written correctly
	s.Equal(string(localContent), mockFile.String())

	s.sshClient.AssertExpectations(s.T())
	mockSFTP.AssertExpectations(s.T())
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
			s.sshClient.On("NewSession").Return(s.sshSession, nil)
			s.sshClient.On("IsConnected").Return(true)
			s.sshSession.On("Run", tt.expectedCmd).Return(nil)
			s.sshSession.On("Close").Return(nil)

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

			s.sshClient.AssertExpectations(s.T())
			s.sshSession.AssertExpectations(s.T())
		})
	}
}

func TestSSHUtilsSuite(t *testing.T) {
	suite.Run(t, new(PkgSSHUtilsTestSuite))
}
