package sshutils

import (
	"fmt"
	"os"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewSSHConfig(t *testing.T) {
	_, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	host := "example.com"
	port := 22
	user := "testuser"
	mockDialer := &MockSSHDialer{}
	config, err := NewSSHConfig(host, port, user, testSSHPrivateKeyPath)
	config.SSHDialer = mockDialer

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, host, config.Host)
	assert.Equal(t, port, config.Port)
	assert.Equal(t, user, config.User)
	assert.NotEmpty(t, config.PrivateKeyPath)
	assert.IsType(t, &logger.Logger{}, config.Logger)
	assert.Equal(t, mockDialer, config.SSHDialer)
}

func TestConnect(t *testing.T) {
	mockDialer := NewMockSSHDialer()
	mockClient, _ := NewMockSSHClient(mockDialer)
	_, _, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPrivateKey()

	config := &SSHConfig{
		Host:           "example.com",
		Port:           22,
		User:           "testuser",
		PrivateKeyPath: testSSHPrivateKeyPath,
		SSHDialer:      mockDialer,
		Logger:         logger.Get(),
	}

	// Set up the mock expectation
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(mockClient, nil)

	client, err := config.Connect()
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.IsType(t, &MockSSHClient{}, client)

	mockDialer.AssertExpectations(t)
}

func TestConnectFailure(t *testing.T) {
	_, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockDialer := NewMockSSHDialer()
	config, _ := NewSSHConfig("example.com", 22, "testuser", testSSHPrivateKeyPath)
	config.SSHDialer = mockDialer
	config.InsecureIgnoreHostKey = true

	expectedError := fmt.Errorf("connection error")
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(nil, expectedError)

	client, err := config.Connect()

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, "failed to connect to SSH server: connection error", err.Error())
	mockDialer.AssertExpectations(t)
}

func TestExecuteCommand(t *testing.T) {
	log := logger.Get()
	mockSSHSession := &MockSSHSession{}

	mockSSHClient, sshConfig := GetTypedMockClient(t, log)
	mockSSHClient.On("NewSession").Return(mockSSHSession, nil)
	NewSSHClientFunc = MockSSHClientCreator(mockSSHClient)

	expectedOutput := []byte("command output")
	mockSSHSession.On("CombinedOutput", "ls -l").Return(expectedOutput, nil)
	mockSSHSession.On("Close").Return(nil)

	// Execute
	actualResult, err := sshConfig.ExecuteCommand("ls -l")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, string(expectedOutput), actualResult)

	mockSSHClient.AssertExpectations(t)
	mockSSHSession.AssertExpectations(t)
}

func TestExecuteCommandWithRetry(t *testing.T) {
	log := logger.Get()
	mockSSHSession := &MockSSHSession{}

	mockSSHClient, sshConfig := GetTypedMockClient(t, log)
	mockSSHClient.On("NewSession").Return(mockSSHSession, nil)
	NewSSHClientFunc = MockSSHClientCreator(mockSSHClient)

	expectedOutput := []byte("command output")
	mockSSHSession.On("CombinedOutput", "ls -l").
		Return(nil, fmt.Errorf("temporary error")).Once().
		On("CombinedOutput", "ls -l").Return(expectedOutput, nil).Once()
	mockSSHSession.On("Close").Return(nil)

	// Execute
	actualResult, err := sshConfig.ExecuteCommand("ls -l")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, string(expectedOutput), actualResult)

	mockSSHClient.AssertExpectations(t)
	mockSSHSession.AssertNumberOfCalls(t, "CombinedOutput", 2)
	mockSSHSession.AssertExpectations(t)
}

func TestPushFile(t *testing.T) {
	log := logger.Get()
	mockClient, _ := GetTypedMockClient(t, log)

	// Create test content
	localContent := []byte("test file content")

	// Mock NewSession to return a mock session
	mockSession := &MockSSHSession{}
	mockClient, sshConfig := GetTypedMockClient(t, log)
	mockClient.On("NewSession").Return(mockSession, nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	// Mock session behavior for file push
	remoteCmd := fmt.Sprintf("cat > %s", "/remote/path")
	mockStdin := &MockWriteCloser{}
	mockSession.On("StdinPipe").Return(mockStdin, nil)
	mockSession.On("Start", remoteCmd).Return(nil)
	mockStdin.On("Write", localContent).Return(len(localContent), nil)
	mockStdin.On("Close").Return(nil)
	mockSession.On("Wait").Return(nil)
	mockSession.On("Close").Return(nil)

	// Test successful file push
	err := sshConfig.PushFile(localContent, "/remote/path", false)
	assert.NoError(t, err)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
	mockStdin.AssertExpectations(t)
}

func TestPushFileExecutable(t *testing.T) {
	log := logger.Get()
	mockClient, _ := GetTypedMockClient(t, log)

	// Create test content
	localContent := []byte("#!/bin/bash\necho 'Hello, World!'")

	// Mock NewSession to return a mock session
	mockSession := &MockSSHSession{}
	mockClient, sshConfig := GetTypedMockClient(t, log)
	mockClient.On("NewSession").Return(mockSession, nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	// Mock session behavior for file push
	remoteCmd := fmt.Sprintf("cat > %s && chmod +x %s", "/remote/path", "/remote/path")
	mockStdin := &MockWriteCloser{}
	mockSession.On("StdinPipe").Return(mockStdin, nil)
	mockSession.On("Start", remoteCmd).Return(nil)
	mockStdin.On("Write", localContent).Return(len(localContent), nil)
	mockStdin.On("Close").Return(nil)
	mockSession.On("Wait").Return(nil)
	mockSession.On("Close").Return(nil)

	// Test successful file push with executable flag
	err := sshConfig.PushFile(localContent, "/remote/path", true)
	assert.NoError(t, err)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
	mockStdin.AssertExpectations(t)
}

func TestInstallSystemdServiceSuccess(t *testing.T) {
	log := logger.Get()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	// Test successful service installation
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", mock.AnythingOfType("string")).Return(assert.AnError)
	mockSession.On("Close").Return(nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	err := sshConfig.InstallSystemdService("service_name", "service_content")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestInstallSystemdServiceFailure(t *testing.T) {
	log := logger.Get()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", mock.AnythingOfType("string")).Return(assert.AnError)
	mockSession.On("Close").Return(nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	err := sshConfig.InstallSystemdService("service_name", "service_content")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestStartServiceSuccess(t *testing.T) {
	log := logger.Get()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	// Test successful service start
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl start service_name").Return(nil)
	mockSession.On("Close").Return(nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	err := sshConfig.StartService("service_name")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestStartServiceFailure(t *testing.T) {
	log := logger.Get()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl start service_name").Return(assert.AnError)
	mockSession.On("Close").Return(nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	err := sshConfig.StartService("service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestRestartServiceSuccess(t *testing.T) {
	log := logger.Get()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	// Test successful service restart
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl restart service_name").Return(nil)
	mockSession.On("Close").Return(nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	err := sshConfig.RestartService("service_name")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestRestartServiceFailure(t *testing.T) {
	log := logger.Get()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl restart service_name").Return(assert.AnError)
	mockSession.On("Close").Return(nil)
	NewSSHClientFunc = MockSSHClientCreator(mockClient)

	err := sshConfig.RestartService("service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func GetMockClient(t *testing.T) (SSHClienter, *SSHConfig) {
	mockDialer := &MockSSHDialer{}
	config, err := NewSSHConfig("example.com", 22, "testuser", "test-key-path")
	if err != nil {
		assert.Fail(t, "failed to create SSH config: %v", err)
	}
	config.SSHDialer = mockDialer

	mockClient := &MockSSHClient{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(mockClient, nil)
	return mockClient, config
}

var _ SSHClienter = (*MockSSHClient)(nil)
