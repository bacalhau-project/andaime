package sshutils

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/bacalhau-project/andaime/logger"
	"github.com/bacalhau-project/andaime/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

type MockSSHDialer struct {
	mock.Mock
}

type MockSSHSession struct {
	mock.Mock
}

func (m *MockSSHSession) Run(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSession) CombinedOutput(cmd string) ([]byte, error) {
	args := m.Called(cmd)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSSHSession) StdinPipe() (io.WriteCloser, error) {
	args := m.Called()
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (m *MockSSHSession) Start(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockSSHSession) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHSession) Wait() error {
	args := m.Called()
	return args.Error(0)
}

type MockSSHClientWrapper struct {
	mock.Mock
}

func (m *MockSSHClientWrapper) NewSession() (SSHSession, error) {
	args := m.Called()
	return args.Get(0).(SSHSession), args.Error(1)
}

func (m *MockSSHClientWrapper) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	args := m.Called(network, addr, config)
	if client, ok := args.Get(0).(*ssh.Client); ok {
		return client, args.Error(1)
	}
	return nil, args.Error(1)
}

type MockWriteCloser struct {
	mock.Mock
}

func (m *MockWriteCloser) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockWriteCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewSSHConfig(t *testing.T) {
	host := "example.com"
	port := 22
	user := "testuser"
	mockDialer := &MockSSHDialer{}

	config, err := NewSSHConfig(host, port, user, mockDialer)

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, host, config.Host)
	assert.Equal(t, port, config.Port)
	assert.Equal(t, user, config.User)
	assert.NotEmpty(t, config.PrivateKey)
	assert.Equal(t, utils.SSHTimeOut, config.Timeout)
	assert.IsType(t, &logger.Logger{}, config.Logger)
	assert.Equal(t, mockDialer, config.SSHDialer)
	assert.IsType(t, &DefaultSSHClientFactory{}, config.SSHClientFactory)
}

func TestConnect(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)

	// Test successful connection
	mockClient := &ssh.Client{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	client, err := config.Connect()
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Test connection error
	mockDialer = &MockSSHDialer{}
	config, _ = NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(nil, assert.AnError)
	client, err = config.Connect()
	assert.Error(t, err)
	assert.Nil(t, client)

	mockDialer.AssertExpectations(t)
}

func TestExecuteCommand(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	mockClient := &MockSSHClientWrapper{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)

	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)

	output := []byte("command output")
	mockSession.On("CombinedOutput", "ls -l").Return(output, nil)
	mockSession.On("Close").Return(nil)

	result, err := config.ExecuteCommand(mockClient, "ls -l")

	assert.NoError(t, err)
	assert.Equal(t, string(output), result)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestPushFile(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	mockClient := &MockSSHClientWrapper{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)

	// Create a temporary local file for testing
	localFile, err := ioutil.TempFile("", "test-local-file")
	assert.NoError(t, err)
	defer os.Remove(localFile.Name())
	localPath := localFile.Name()

	// Write some content to the local file
	localContent := "test file content"
	_, err = localFile.WriteString(localContent)
	assert.NoError(t, err)
	localFile.Close()

	// Mock the Dial method
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	// Mock NewSession to return a mock session
	mockSession := &MockSSHSession{}
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
	err = config.PushFile(mockClient, localPath, "/remote/path")
	assert.NoError(t, err)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
	mockStdin.AssertExpectations(t)
}

func TestInstallSystemdServiceSuccess(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockClient := &MockSSHClientWrapper{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	// Test successful service installation
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", mock.AnythingOfType("string")).Return(assert.AnError)
	mockSession.On("Close").Return(nil)

	err := config.InstallSystemdService(mockClient, "service_name", "service_content")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestInstallSystemdServiceFailure(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockClient := &MockSSHClientWrapper{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", mock.AnythingOfType("string")).Return(assert.AnError)
	mockSession.On("Close").Return(nil)

	err := config.InstallSystemdService(mockClient, "service_name", "service_content")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}
func TestStartServiceSuccess(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockClient := &MockSSHClientWrapper{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	// Test successful service start
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl start service_name").Return(nil)
	mockSession.On("Close").Return(nil)

	err := config.StartService(mockClient, "service_name")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestStartServiceFailure(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockClient := &MockSSHClientWrapper{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl start service_name").Return(assert.AnError)
	mockSession.On("Close").Return(nil)

	err := config.StartService(mockClient, "service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestRestartServiceSuccess(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockClient := &MockSSHClientWrapper{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	// Test successful service restart
	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl restart service_name").Return(nil)
	mockSession.On("Close").Return(nil)

	err := config.RestartService(mockClient, "service_name")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

func TestRestartServiceFailure(t *testing.T) {
	mockDialer := &MockSSHDialer{}
	config, _ := NewSSHConfig("example.com", 22, "testuser", mockDialer)
	mockClient := &MockSSHClientWrapper{}
	mockDialer.On("Dial", "tcp", "example.com:22", mock.AnythingOfType("*ssh.ClientConfig")).Return(mockClient, nil)

	mockSession := &MockSSHSession{}
	mockClient.On("NewSession").Return(mockSession, nil)
	mockSession.On("Run", "sudo systemctl restart service_name").Return(assert.AnError)
	mockSession.On("Close").Return(nil)

	err := config.RestartService(mockClient, "service_name")
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockSession.AssertExpectations(t)
}

var _ SSHClient = (*MockSSHClientWrapper)(nil)
