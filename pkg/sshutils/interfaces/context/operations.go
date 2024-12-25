package context

import (
"context"
"io"
"time"

"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces/types"
)

// SSHOperations defines context-dependent SSH operations
type SSHOperations interface {
Connect(ctx context.Context) error
RunCommand(ctx context.Context, command string) (string, error)
RunCommandWithTimeout(ctx context.Context, command string, timeout time.Duration) (string, error)
CopyFile(ctx context.Context, source io.Reader, destination string, mode int) error
IsConnected(ctx context.Context) bool
NewSession(ctx context.Context) (types.SSHSessioner, error)
}
