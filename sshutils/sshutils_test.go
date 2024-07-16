package sshutils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/bacalhau-project/andaime/logger"
	"golang.org/x/crypto/ssh"
)

type mockSSHClient struct {
	newSessionFunc func() (SSHSession, error)
	closeFunc      func() error
}

func (m *mockSSHClient) NewSession() (SSHSession, error) {
	return m.newSessionFunc()
}

func (m *mockSSHClient) Close() error {
	return m.closeFunc()
}

type mockSSHSession struct {
	runFunc            func(cmd string) error
	combinedOutputFunc func(cmd string) ([]byte, error)
	closeFunc          func() error
}

func (m *mockSSHSession) Run(cmd string) error {
	return m.runFunc(cmd)
}

func (m *mockSSHSession) CombinedOutput(cmd string) ([]byte, error) {
	return m.combinedOutputFunc(cmd)
}

func (m *mockSSHSession) Close() error {
	return m.closeFunc()
}

type mockSSHClientFactory struct {
	newSSHClientFunc func(network, addr string, config *ssh.ClientConfig) (SSHClient, error)
}

func (f *mockSSHClientFactory) NewSSHClient(network, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	return f.newSSHClientFunc(network, addr, config)
}

func TestConnect(t *testing.T) {
	logger.InitTest(t)

	// Generate a test RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Convert the private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateKeyPEMBytes := pem.EncodeToMemory(privateKeyPEM)

	mockFactory := &mockSSHClientFactory{
		newSSHClientFunc: func(network, addr string, config *ssh.ClientConfig) (SSHClient, error) {
			return &mockSSHClient{
				newSessionFunc: func() (SSHSession, error) {
					return &mockSSHSession{}, nil
				},
				closeFunc: func() error {
					return nil
				},
			}, nil
		},
	}

	config := &SSHConfig{
		Host:             "example.com",
		Port:             22,
		User:             "user",
		PrivateKey:       string(privateKeyPEMBytes),
		Logger:           logger.Get(),
		SSHClientFactory: mockFactory,
	}

	client, err := config.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
}

func TestExecuteCommand(t *testing.T) {
	logger.InitTest(t)
	mockClient := &mockSSHClient{
		newSessionFunc: func() (SSHSession, error) {
			return &mockSSHSession{
				combinedOutputFunc: func(cmd string) ([]byte, error) {
					return []byte("success"), nil
				},
				closeFunc: func() error {
					return nil
				},
			}, nil
		},
	}

	config := &SSHConfig{
		Host:   "example.com",
		Port:   22,
		User:   "user",
		Logger: logger.Get(),
	}

	output, err := config.ExecuteCommand(mockClient, "test command")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}
	if output != "success" {
		t.Fatalf("Expected output 'success', got '%s'", output)
	}
}

func TestExecuteCommandWithRetry(t *testing.T) {
	logger.InitTest(t)
	sessionCallCount := 0
	mockClient := &mockSSHClient{
		newSessionFunc: func() (SSHSession, error) {
			sessionCallCount++
			if sessionCallCount < 3 {
				return nil, errors.New("network error")
			}
			return &mockSSHSession{
				combinedOutputFunc: func(cmd string) ([]byte, error) {
					return []byte("success"), nil
				},
				closeFunc: func() error {
					return nil
				},
			}, nil
		},
	}

	config := &SSHConfig{
		Host:   "example.com",
		Port:   22,
		User:   "user",
		Logger: logger.Get(),
	}

	output, err := config.ExecuteCommand(mockClient, "test command")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}
	if output != "success" {
		t.Fatalf("Expected output 'success', got '%s'", output)
	}
	if sessionCallCount != 3 {
		t.Fatalf("Expected 3 session calls, got %d", sessionCallCount)
	}
}

// Add more tests for other methods like PushFile, InstallSystemdService, etc.
