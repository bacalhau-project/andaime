package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/mocks"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

func TestExecuteCustomScript(t *testing.T) {
	tests := []struct {
		name             string
		customScriptPath string
		setupMocks       func(*mocks.MockMachiner, *mocks.MockSSHClient)
		expectedOutput   string
		expectedError    string
	}{
		{
			name:             "Successful script execution",
			customScriptPath: "/path/to/valid/script.sh",
			setupMocks: func(mockMachine *mocks.MockMachiner, mockSSHClient *mocks.MockSSHClient) {
				mockMachine.On("GetName").Return("test-machine")
				mockMachine.On("GetPublicIP").Return("1.2.3.4")
				mockSSHClient.On("ExecuteCommandWithContext", mock.Anything, "bash /path/to/valid/script.sh").Return("Script output", nil)
			},
			expectedOutput: "Script output",
			expectedError:  "",
		},
		{
			name:             "Script execution failure",
			customScriptPath: "/path/to/valid/script.sh",
			setupMocks: func(mockMachine *mocks.MockMachiner, mockSSHClient *mocks.MockSSHClient) {
				mockMachine.On("GetName").Return("test-machine")
				mockMachine.On("GetPublicIP").Return("1.2.3.4")
				mockSSHClient.On("ExecuteCommandWithContext", mock.Anything, "bash /path/to/valid/script.sh").Return("", &ssh.ExitError{Waitmsg: ssh.ExitMissingError{}})
			},
			expectedOutput: "",
			expectedError:  "custom script execution failed with exit code -1",
		},
		{
			name:             "Script execution timeout",
			customScriptPath: "/path/to/valid/script.sh",
			setupMocks: func(mockMachine *mocks.MockMachiner, mockSSHClient *mocks.MockSSHClient) {
				mockMachine.On("GetName").Return("test-machine")
				mockMachine.On("GetPublicIP").Return("1.2.3.4")
				mockSSHClient.On("ExecuteCommandWithContext", mock.Anything, "bash /path/to/valid/script.sh").After(6*time.Minute).Return("", context.DeadlineExceeded)
			},
			expectedOutput: "",
			expectedError:  "custom script execution timed out",
		},
		{
			name:             "Empty custom script path",
			customScriptPath: "",
			setupMocks: func(mockMachine *mocks.MockMachiner, mockSSHClient *mocks.MockSSHClient) {
				mockMachine.On("GetName").Return("test-machine")
			},
			expectedOutput: "",
			expectedError:  "custom script path is empty",
		},
		{
			name:             "Non-existent custom script",
			customScriptPath: "/path/to/non-existent/script.sh",
			setupMocks: func(mockMachine *mocks.MockMachiner, mockSSHClient *mocks.MockSSHClient) {
				mockMachine.On("GetName").Return("test-machine")
			},
			expectedOutput: "",
			expectedError:  "invalid custom script: custom script does not exist: /path/to/non-existent/script.sh",
		},
		{
			name:             "Nil machine",
			customScriptPath: "/path/to/valid/script.sh",
			setupMocks:       func(mockMachine *mocks.MockMachiner, mockSSHClient *mocks.MockSSHClient) {},
			expectedOutput:   "",
			expectedError:    "machine is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMachine := &mocks.MockMachiner{}
			mockSSHClient := &mocks.MockSSHClient{}
			tt.setupMocks(mockMachine, mockSSHClient)

			// Create a temporary file for the custom script if it's supposed to exist
			if tt.customScriptPath != "" && !strings.Contains(tt.name, "Non-existent") {
				tmpfile, err := os.CreateTemp("", "testscript")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpfile.Name())
				tt.customScriptPath = tmpfile.Name()
			}

			deployment := &models.Deployment{
				SSHUser:           "testuser",
				SSHPublicKeyPath:  "/path/to/public/key",
				SSHPrivateKeyPath: "/path/to/private/key",
				SSHPort:           22,
			}

			cd := &ClusterDeployer{}

			// Mock the NewSSHClient function
			origNewSSHClient := sshutils.NewSSHClient
			sshutils.NewSSHClient = func(user, pubKeyPath, privKeyPath, host string, port int) (*sshutils.SSHClient, error) {
				return mockSSHClient, nil
			}
			defer func() { sshutils.NewSSHClient = origNewSSHClient }()

			output, err := cd.ExecuteCustomScript(context.Background(), deployment, mockMachine, tt.customScriptPath)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedOutput, output)

			mockMachine.AssertExpectations(t)
			mockSSHClient.AssertExpectations(t)
		})
	}
}
