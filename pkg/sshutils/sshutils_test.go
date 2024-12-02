package sshutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

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
	s.sshClient.On("IsConnected").Return(true).Maybe()
}

func (s *PkgSSHUtilsTestSuite) TestConnect() {
	mockSSHClient := &ssh.Client{}
	originalDialSSH := DialSSHFunc
	DialSSHFunc = func(network,
		addr string,
		config *ssh.ClientConfig) (sshutils_interfaces.SSHClienter, error) {
		return &SSHClientWrapper{Client: mockSSHClient}, nil
	}
	defer func() { DialSSHFunc = originalDialSSH }()

	client, err := s.sshConfig.Connect()
	s.NoError(err)
	s.NotNil(client)
	s.IsType(&SSHClientWrapper{}, client)

	s.sshClient.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestConnectFailure() {
	expectedError := fmt.Errorf("connection error")
	SSHRetryDelay = 0 * time.Second
	originalDialSSH := DialSSHFunc
	DialSSHFunc = func(network,
		addr string,
		config *ssh.ClientConfig) (sshutils_interfaces.SSHClienter, error) {
		return nil, expectedError
	}
	defer func() { DialSSHFunc = originalDialSSH }()

	client, err := s.sshConfig.Connect()
	s.Error(err)
	s.Nil(client)
	s.Contains(err.Error(), "failed to connect")
	s.sshClient.AssertExpectations(s.T())
}

func (s *PkgSSHUtilsTestSuite) TestExecuteCommand() {
	expectedOutput := []byte("command output")

	mockSession := &ssh_mock.MockSSHSessioner{}
	mockSession.On("CombinedOutput", "ls -l").Return(expectedOutput, nil).Once()
	mockSession.On("Close").Return(nil).Once()
	s.sshClient.On("NewSession").Return(mockSession, nil).Once()

	s.sshConfig.SetSSHClienter(s.sshClient)

	actualResult, err := s.sshConfig.ExecuteCommand(s.ctx, "ls -l")
	s.NoError(err)
	s.Equal(string(expectedOutput), actualResult)

	s.sshClient.AssertExpectations(s.T())
	mockSession.AssertExpectations(s.T())
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
	mockSFTP.On("Create", "/remote/path").Return(mockFile, nil).Once()
	if executable {
		mockSFTP.On("Chmod", "/remote/path", os.FileMode(0755)).Return(nil).Once()
	}
	mockSFTP.On("Close").Return(nil).Once()

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
	s.sshClient.On("GetClient").Return(mockSSHClient).Once()

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
			// Mock GetClient() to return a mock SSH client
			mockSSHClient := &ssh.Client{}
			mockSession := &ssh_mock.MockSSHSessioner{}
			mockSession.On("Close").Return(nil)

			s.sshClient.On("GetClient").Return(mockSSHClient).Once()
			s.sshClient.On("NewSession").Return(mockSession, nil)

			// Mock SFTP client creation
			mockSFTP := &mockSFTPClient{}
			mockSFTP.On("Create", mock.Anything).Return(&mockWriteCloser{
				Buffer: bytes.NewBuffer(nil),
				closeFunc: func() error {
					return nil
				},
			}, nil).Maybe()
			mockSFTP.On("Close").Return(nil).Maybe()
			mockSFTP.On("Chmod", mock.Anything, mock.Anything).Return(nil).Maybe()

			// Override the SFTP client creator for testing
			originalCreator := DefaultSFTPClientCreator
			DefaultSFTPClientCreator = func(client *ssh.Client) (sshutils_interfaces.SFTPClienter, error) {
				return mockSFTP, nil
			}
			defer func() { DefaultSFTPClientCreator = originalCreator }()

			mockSession.On("CombinedOutput", mock.Anything).Return([]byte(""), nil).Maybe()

			var err error
			var result string
			switch op := tt.operation.(type) {
			case func(context.Context, string, string) error:
				err = op(s.ctx, tt.serviceName, tt.serviceContent)
			case func(context.Context, string) (string, error):
				result, err = op(s.ctx, tt.serviceName)
				s.NoError(err)
				if tt.serviceContent != "" {
					s.NotEmpty(result)
					s.Contains(result, tt.serviceContent)
				}
			default:
				s.Fail("Unexpected operation type")
			}

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}

			mockSFTP.AssertExpectations(s.T())
		})
	}
}

func TestSSHUtilsSuite(t *testing.T) {
	suite.Run(t, new(PkgSSHUtilsTestSuite))
}
