package sshutils

import (
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

var SSHDialerFunc = NewSSHDial

type SSHDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error)
}

func NewSSHDial(host string, port int, config *ssh.ClientConfig) SSHDialer {
	return &SSHDial{
		DialCreator: func(network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
			client, err := ssh.Dial(network, addr, config)
			if err != nil {
				return nil, err
			}
			return &SSHClientWrapper{Client: client}, nil
		},
	}
}

type SSHDial struct {
	DialCreator func(network, addr string, config *ssh.ClientConfig) (SSHClienter, error)
}

func (d *SSHDial) Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
	return d.DialCreator(network, addr, config)
}

// Mock Functions

// MockSSHDialer is a mock implementation of SSHDialer
type MockSSHDialer struct {
	mock.Mock
}

// Dial is a mock implementation of the Dial method
func (m *MockSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (SSHClienter, error) {
	args := m.Called(network, addr, config)
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(SSHClienter), nil
}

// NewMockSSHDialer returns a MockSSHDialer with a default implementation
func NewMockSSHDialer() *MockSSHDialer {
	return &MockSSHDialer{}
}

func NewMockSSHClient(dialer SSHDialer) (*MockSSHClient, *SSHConfig) {
	_, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockDialer := &MockSSHDialer{}
	config, err := NewSSHConfig("example.com", 22, "testuser", mockDialer, testSSHPrivateKeyPath) //nolint:gomnd
	if err != nil {
		panic(err)
	}
	mockClient := &MockSSHClient{}
	return mockClient, config
}
