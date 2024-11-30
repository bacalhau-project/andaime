package sshutils

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"

	internal_testutil "github.com/bacalhau-project/andaime/internal/testutil"
)

var SSHDialerFunc = NewSSHDial

// SSHDialer is now defined in interfaces.go

type sshDial struct {
	host   string
	port   int
	config *ssh.ClientConfig
}

func NewSSHDial(host string, port int, config *ssh.ClientConfig) SSHDialer {
	return &sshDial{
		host:   host,
		port:   port,
		config: config,
	}
}

func (s *sshDial) Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
	client, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	return &SSHClientWrapper{Client: client}, nil
}

func (s *sshDial) DialContext(ctx context.Context, network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
	type dialResult struct {
		client SSHClienter
		err    error
	}

	result := make(chan dialResult, 1)

	// Start dialing in a goroutine
	go func() {
		client, err := s.Dial(network, addr, config)
		result <- dialResult{client, err}
	}()

	// Wait for either dial completion or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-result:
		return res.client, res.err
	}
}

// Mock Functions

// MockSSHDialer is a mock implementation of SSHDialer for testing
type MockSSHDialer struct {
	DialFunc func(network, addr string, config *ssh.ClientConfig) (SSHClienter, error)
}

func (m *MockSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
	if m.DialFunc != nil {
		return m.DialFunc(network, addr, config)
	}
	return nil, nil
}

func (m *MockSSHDialer) DialContext(
	ctx context.Context,
	network, addr string,
	config *ssh.ClientConfig,
) (SSHClienter, error) {
	args := m.Called(ctx, network, addr, config)
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SSHClienter), nil
}

// NewMockSSHDialer returns a MockSSHDialer with a default implementation
func NewMockSSHDialer() *MockSSHDialer {
	return &MockSSHDialer{}
}

func NewMockSFTPClient() *MockSFTPClient {
	return &MockSFTPClient{}
}

func NewMockSSHClient(dialer SSHDialer) (*MockSSHClient, SSHConfiger) {
	_,
		cleanupPublicKey,
		testSSHPrivateKeyPath,
		cleanupPrivateKey := internal_testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	configInterface, err := NewSSHConfigFunc(
		"example.com",
		22, //nolint:mnd
		"testuser",
		testSSHPrivateKeyPath,
	) //nolint:mnd
	if err != nil {
		panic(err)
	}
	config := configInterface
	config.SetSSHClienter(&MockSSHClient{})

	mockClient := &MockSSHClient{}
	return mockClient, config
}
