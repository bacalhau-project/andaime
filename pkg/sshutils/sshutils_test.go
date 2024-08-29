package sshutils

import (
	"context"
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

	testSSHPrivateKeyMaterial, err := os.ReadFile(testSSHPrivateKeyPath)
	if err != nil {
		panic(err)
	}

	host := "example.com"
	port := 22
	user := "testuser"
	mockDialer := &MockSSHDialer{}
	configInterface, err := NewSSHConfigFunc(host, port, user, testSSHPrivateKeyMaterial)
	config := configInterface.(*SSHConfig)
	config.SSHDial = mockDialer

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, host, config.Host)
	assert.Equal(t, port, config.Port)
	assert.Equal(t, user, config.User)
	assert.NotEmpty(t, config.PrivateKeyMaterial)
	assert.IsType(t, &logger.Logger{}, config.Logger)
	assert.Equal(t, mockDialer, config.SSHDial)
}

func TestConnect(t *testing.T) {
	mockDialer := NewMockSSHDialer()
	mockClient, _ := NewMockSSHClient(mockDialer)
	_, _, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPrivateKey()

	testSSHPrivateKeyMaterial, err := os.ReadFile(testSSHPrivateKeyPath)
	if err != nil {
		panic(err)
	}

	config := &SSHConfig{
		Host:               "example.com",
		Port:               22,
		User:               "testuser",
		PrivateKeyMaterial: testSSHPrivateKeyMaterial,
		SSHDial:            mockDialer,
		Logger:             logger.Get(),
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

	testSSHPrivateKeyMaterial, err := os.ReadFile(testSSHPrivateKeyPath)
	if err != nil {
		panic(err)
	}

	mockDialer := NewMockSSHDialer()
	configInterface, _ := NewSSHConfigFunc("example.com", 22, "testuser", testSSHPrivateKeyMaterial)
	config := configInterface.(*SSHConfig)
	config.SSHDial = mockDialer
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
	ctx := context.Background()
	mockSSHSession := &MockSSHSession{}

	mockSSHClient, sshConfig := GetTypedMockClient(t, log)
	mockSSHClient.On("NewSession").Return(mockSSHSession, nil)

	expectedOutput := []byte("command output")
	mockSSHSession.On("CombinedOutput", "ls -l").Return(expectedOutput, nil)
	mockSSHSession.On("Close").Return(nil)

	sshConfig.SetSSHClient(mockSSHClient)
	// Execute
	actualResult, err := sshConfig.ExecuteCommand(ctx, "ls -l")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, string(expectedOutput), actualResult)

	mockSSHClient.AssertExpectations(t)
	mockSSHSession.AssertExpectations(t)
}

func TestExecuteCommandWithRetry(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockSSHSession := &MockSSHSession{}

	mockSSHClient, sshConfig := GetTypedMockClient(t, log)
	mockSSHClient.On("NewSession").Return(mockSSHSession, nil)

	expectedOutput := []byte("command output")
	mockSSHSession.On("CombinedOutput", "ls -l").
		Return([]byte{}, fmt.Errorf("temporary error")).Once().
		On("CombinedOutput", "ls -l").Return(expectedOutput, nil).Once()
	mockSSHSession.On("Close").Return(nil).Times(2)
	mockSSHSession.On("Close").Return(nil)

	sshConfig.SetSSHClient(mockSSHClient)

	// Execute
	actualResult, err := sshConfig.ExecuteCommand(ctx, "ls -l")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, string(expectedOutput), actualResult)

	mockSSHClient.AssertExpectations(t)
	mockSSHSession.AssertNumberOfCalls(t, "CombinedOutput", 2)
	mockSSHSession.AssertExpectations(t)
}

func TestPushFile(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, _ := GetTypedMockClient(t, log)

	// Create test content
	localContent := []byte("test file content")

	// Mock NewSession to return a mock session
	mockSession := &MockSSHSession{}
	mockClient, sshConfig := GetTypedMockClient(t, log)
	mockClient.On("NewSession").Return(mockSession, nil)

	// Mock session behavior for file push
	remoteCmd := fmt.Sprintf("cat > %s", "/remote/path")
	mockStdin := &MockWriteCloser{}
	mockSession.On("StdinPipe").Return(mockStdin, nil)
	mockSession.On("Start", remoteCmd).Return(nil)
	mockStdin.On("Write", localContent).Return(len(localContent), nil)
	mockStdin.On("Close").Return(nil)
	mockSession.On("Wait").Return(nil)
	mockSession.On("Close").Return(nil)

	sshConfig.SetSSHClient(mockClient)

	// Test successful file push
	err := sshConfig.PushFile(ctx, "/remote/path", localContent, false)
	assert.NoError(t, err)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
	mockStdin.AssertExpectations(t)
}

func TestPushFileExecutable(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, _ := GetTypedMockClient(t, log)

	// Create test content
	localContent := []byte("#!/bin/bash\necho 'Hello, World!'")

	// Mock NewSession to return a mock session
	mockSession := &MockSSHSession{}
	mockClient, sshConfig := GetTypedMockClient(t, log)
	mockClient.On("NewSession").Return(mockSession, nil)

	// Mock session behavior for file push
	remoteCmd := fmt.Sprintf("cat > %s && chmod +x %s", "/remote/path", "/remote/path")
	mockStdin := &MockWriteCloser{}
	mockSession.On("StdinPipe").Return(mockStdin, nil)
	mockSession.On("Start", remoteCmd).Return(nil)
	mockStdin.On("Write", localContent).Return(len(localContent), nil)
	mockStdin.On("Close").Return(nil)
	mockSession.On("Wait").Return(nil)
	mockSession.On("Close").Return(nil)

	sshConfig.SetSSHClient(mockClient)

	// Test successful file push with executable flag
	err := sshConfig.PushFile(ctx, "/remote/path", localContent, true)
	assert.NoError(t, err)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
	mockStdin.AssertExpectations(t)
}

func TestInstallSystemdServiceSuccess(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	// Test successful service installation
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", mock.AnythingOfType("string")).Return(assert.AnError)
	mockSession.On("Close").Return(nil)

	sshConfig.SetSSHClient(mockClient)

	err := sshConfig.InstallSystemdService(ctx, "service_name", "service_content")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestInstallSystemdServiceFailure(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", mock.AnythingOfType("string")).Return(assert.AnError)
	mockSession.On("Close").Return(nil)

	sshConfig.SetSSHClient(mockClient)

	err := sshConfig.InstallSystemdService(ctx, "service_name", "service_content")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestStartServiceSuccess(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	// Test successful service start
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl start service_name").Return(nil)
	mockSession.On("Close").Return(nil)
	sshConfig.SetSSHClient(mockClient)

	err := sshConfig.StartService(ctx, "service_name")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestStartServiceFailure(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl start service_name").Return(assert.AnError)
	mockSession.On("Close").Return(nil)
	sshConfig.SetSSHClient(mockClient)

	err := sshConfig.StartService(ctx, "service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestRestartServiceSuccess(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	// Test successful service restart
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl restart service_name").Return(nil)
	mockSession.On("Close").Return(nil)
	sshConfig.SetSSHClient(mockClient)

	err := sshConfig.RestartService(ctx, "service_name")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestRestartServiceFailure(t *testing.T) {
	log := logger.Get()
	ctx := context.Background()
	mockClient, sshConfig := GetTypedMockClient(t, log)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl restart service_name").Return(assert.AnError)
	mockSession.On("Close").Return(nil)
	sshConfig.SetSSHClient(mockClient)

	err := sshConfig.RestartService(ctx, "service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func GetMockClient(t *testing.T) (SSHClienter, *SSHConfig) {
	mockDialer := &MockSSHDialer{}
	configInterface, err := NewSSHConfigFunc(
		"example.com",
		22,
		"testuser",
		[]byte("test-key-material"),
	)
	if err != nil {
		assert.Fail(t, "failed to create SSH config: %v", err)
	}
	config := configInterface.(*SSHConfig)
	config.SSHDial = mockDialer

	mockClient := &MockSSHClient{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).
		Return(mockClient, nil)
	return mockClient, config
}

var _ SSHClienter = (*MockSSHClient)(nil)
