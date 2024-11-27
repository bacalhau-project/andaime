package sshutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
)

type PkgSSHUtilsTestSuite struct {
	suite.Suite
	testSSHPrivateKeyPath string
	cleanupPrivateKey     func()
	mockDialer            *MockSSHDialer
	mockClient            *MockSSHClient
	mockSession           *MockSSHSession
	sshConfig             SSHConfiger
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
	s.sshConfig, _ = NewSSHConfigFunc(
		"example.com",
		22, //nolint:mnd
		"testuser",
		s.testSSHPrivateKeyPath,
	)

	s.sshConfig.SetSSHDial(s.mockDialer)
	s.sshConfig.SetSSHClienter(s.mockClient)

	// Mock the validateSSHConnection method to bypass network checks
	s.sshConfig.SetValidateSSHConnection(func() error {
		return nil
	})
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
	s.Contains(err.Error(), "connection error")
	s.mockDialer.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestConnectInvalidDialerInfo() {
	s.mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(nil, fmt.Errorf("connection error"))

	client, err := s.sshConfig.Connect()
	s.Error(err)
	s.Nil(client)
	s.Contains(err.Error(), "connection error")
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

// Verify that mockSFTPClient implements SFTPClient
var _ SFTPClient = &mockSFTPClient{}

func (m *mockSFTPClient) MkdirAll(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *mockSFTPClient) Create(path string) (SFTPFile, error) {
	args := m.Called(path)
	return args.Get(0).(SFTPFile), args.Error(1)
}

func (m *mockSFTPClient) Chmod(path string, mode os.FileMode) error {
	args := m.Called(path, mode)
	return args.Error(0)
}

func (m *mockSFTPClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockSFTPFile struct {
	mock.Mock
}

func (m *mockSFTPFile) Write(p []byte) (int, error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *mockSFTPFile) Close() error {
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

	// Create mock SFTP file
	mockFile := &mockSFTPFile{}
	mockFile.On("Write", localContent).Return(len(localContent), nil)
	mockFile.On("Close").Return(nil)

	// Create mock SFTP client
	mockSFTP := &mockSFTPClient{}
	mockSFTP.On("MkdirAll", "/remote").Return(nil)
	mockSFTP.On("Create", "/remote/path").Return(mockFile, nil)
	if executable {
		mockSFTP.On("Chmod", "/remote/path", os.FileMode(0700)).Return(nil)
	}
	mockSFTP.On("Close").Return(nil)

	// Save the original creator and restore it after the test
	originalCreator := currentSFTPClientCreator
	defer func() { currentSFTPClientCreator = originalCreator }()

	// Set up our test creator that returns the mock
	var testCreator SFTPClientCreator = func(client *ssh.Client) (SFTPClient, error) {
		return mockSFTP, nil
	}
	currentSFTPClientCreator = testCreator

	// Mock GetClient to return a mock ssh.Client
	mockSSHClient := &ssh.Client{}
	s.mockClient.On("GetClient").Return(mockSSHClient)

	err := s.sshConfig.PushFile(s.ctx, "/remote/path", localContent, executable)
	s.NoError(err)

	s.mockClient.AssertExpectations(s.T())
	mockSFTP.AssertExpectations(s.T())
	mockFile.AssertExpectations(s.T())
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
