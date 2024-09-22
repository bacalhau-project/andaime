package sshutils

import (
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"

	internal_testutil "github.com/bacalhau-project/andaime/internal/testutil"
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

func NewMockSSHClient(dialer SSHDialer) (*MockSSHClient, SSHConfiger) {
	_, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := internal_testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockDialer := &MockSSHDialer{}
	configInterface, err := NewSSHConfigFunc(
		"example.com",
		22, //nolint:mnd
		"testuser",
		testSSHPrivateKeyPath,
	) //nolint:mnd
	if err != nil {
		panic(err)
	}
	config := configInterface.(*SSHConfig)
	config.SSHDial = mockDialer

	mockClient := &MockSSHClient{}
	return mockClient, config
}
