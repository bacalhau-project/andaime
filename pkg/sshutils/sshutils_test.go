package sshutils

import (
	"fmt"
	"os"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testdata"
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
	config, err := NewSSHConfig(host, port, user, mockDialer, testSSHPrivateKeyPath)

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, host, config.Host)
	assert.Equal(t, port, config.Port)
	assert.Equal(t, user, config.User)
	assert.NotEmpty(t, config.PrivateKeyMaterial)
	assert.IsType(t, &logger.Logger{}, config.Logger)
	assert.Equal(t, mockDialer, config.SSHDialer)
}

func TestConnect(t *testing.T) {
	mockDialer := NewMockSSHDialer()
	mockClient, _ := NewMockSSHClient(mockDialer)

	config := &SSHConfig{
		Host:               "example.com",
		Port:               22,
		User:               "testuser",
		PrivateKeyMaterial: testdata.TestPrivateSSHKeyMaterial,
		SSHDialer:          mockDialer,
		Logger:             logger.Get(),
	}

	// Set up the mock expectation
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

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
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer, testSSHPrivateKeyPath)
	config.InsecureIgnoreHostKey = true

	expectedError := fmt.Errorf("connection error")
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(nil, expectedError)

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

	expectedOutput := []byte("command output")
	mockSSHSession.On("CombinedOutput", "ls -l").Return(expectedOutput, nil)
	mockSSHSession.On("Close").Return(nil)

	// Execute
	actualResult, err := sshConfig.ExecuteCommand(mockSSHClient, "ls -l")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, string(expectedOutput), actualResult)

	mockSSHClient.AssertExpectations(t)
	mockSSHSession.AssertExpectations(t)
}

func TestPushFile(t *testing.T) {
	log := logger.Get()
	mockClient, _ := GetTypedMockClient(t, log)

	// Create a temporary local file for testing
	localFile, err := os.CreateTemp("", "test-local-file")
	assert.NoError(t, err)
	defer os.Remove(localFile.Name())
	localPath := localFile.Name()

	// Write some content to the local file
	localContent := "test file content"
	_, err = localFile.WriteString(localContent)
	assert.NoError(t, err)
	localFile.Close()

	// Mock NewSession to return a mock session
	mockSession := &MockSSHSession{}
	mockClient, sshConfig := GetTypedMockClient(t, log)
	mockClient.On("NewSession").Return(mockSession, nil)

	// Mock session behavior for file push
	remoteCmd := fmt.Sprintf("cat > %s", "/remote/path")
	mockStdin := &MockWriteCloser{}
	mockSession.On("StdinPipe").Return(mockStdin, nil)
	mockSession.On("Start", remoteCmd).Return(nil)
	mockStdin.On("Write", []byte(localContent)).Return(len(localContent), nil)
	mockStdin.On("Close").Return(nil)
	mockSession.On("Wait").Return(nil)
	mockSession.On("Close").Return(nil)

	// Test successful file push
	err = sshConfig.PushFile(mockClient, localPath, "/remote/path")
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

	err := sshConfig.InstallSystemdService(mockClient, "service_name", "service_content")
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

	err := sshConfig.InstallSystemdService(mockClient, "service_name", "service_content")
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

	err := sshConfig.StartService(mockClient, "service_name")
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

	err := sshConfig.StartService(mockClient, "service_name")
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

	err := sshConfig.RestartService(mockClient, "service_name")
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

	err := sshConfig.RestartService(mockClient, "service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func GetMockClient(t *testing.T) (SSHClienter, *SSHConfig) {
	mockDialer := &MockSSHDialer{}
	config, err := NewSSHConfig("example.com", 22, "testuser", mockDialer, "test-key-path")
	if err != nil {
		assert.Fail(t, "failed to create SSH config: %v", err)
	}
	mockClient := &MockSSHClient{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)
	return mockClient, config
}

var _ SSHClienter = (*MockSSHClient)(nil)
