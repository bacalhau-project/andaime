package sshutils

import (
	"time"
)

const (
	DefaultSSHPort    = 22
	DefaultRetryCount = 5
	DefaultRetryDelay = 10 * time.Second
)

// type SSHLivenessChecker struct {
// 	RetryCount int
// 	RetryDelay time.Duration
// }

// func NewSSHLivenessChecker() *SSHLivenessChecker {
// 	return &SSHLivenessChecker{
// 		RetryCount: DefaultRetryCount,
// 		RetryDelay: DefaultRetryDelay,
// 	}
// }

// func (c *SSHLivenessChecker) CheckSSHLiveness(
// 	ctx context.Context,
// 	ip, user string,
// 	privateKeyMaterial []byte,
// 	port int,
// ) error {
// 	sshConfig, err := NewSSHConfigFunc(ip, port, user, privateKeyMaterial)
// 	if err != nil {
// 		return fmt.Errorf("failed to get SSH config: %v", err)
// 	}

// 	for i := 0; i < c.RetryCount; i++ {
// 		err = trySSHConnection(ctx, ip, port, sshConfig)
// 		if err == nil {
// 			return nil
// 		}
// 		time.Sleep(c.RetryDelay)
// 	}

// 	return fmt.Errorf("failed to establish SSH connection after %d attempts: %v", c.RetryCount, err)
// }

// func (c *SSHLivenessChecker) TestConnectivity(
// 	ip, user string,
// 	port int,
// 	privateKeyPath string,
// ) error {
// 	sshConfig, err := getSSHConfig(user, privateKeyPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to get SSH config: %v", err)
// 	}

// 	for i := 0; i < c.RetryCount; i++ {
// 		err = testSSHConnection(ip, port, sshConfig)
// 		if err == nil {
// 			return nil
// 		}
// 		time.Sleep(c.RetryDelay)
// 	}

// 	return fmt.Errorf("failed to establish SSH connection after %d attempts: %v", c.RetryCount, err)
// }

// func trySSHConnection(
// 	_ context.Context,
// 	config SSHConfiger,
// ) error {
// 	client, err := config.Connect()
// 	if err != nil {
// 		return err
// 	}
// 	defer client.Close()

// 	return nil
// }

// func testSSHConnection(ip string, port int, config *ssh.ClientConfig) error {
// 	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), config)
// 	if err != nil {
// 		return err
// 	}
// 	defer conn.Close()

// 	session, err := conn.NewSession()
// 	if err != nil {
// 		return err
// 	}
// 	defer session.Close()

// 	output, err := session.CombinedOutput("echo 'SSH connection successful'")
// 	if err != nil {
// 		return fmt.Errorf("failed to execute command: %v", err)
// 	}

// 	if string(output) != "SSH connection successful\n" {
// 		return fmt.Errorf("unexpected output: %s", string(output))
// 	}

// 	return nil
// }
