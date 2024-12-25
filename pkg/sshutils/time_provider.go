package sshutils

import (
	"time"
	"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces"
	"github.com/bacalhau-project/andaime/pkg/sshutils/interfaces/types"
)

// DefaultTimeProvider implements the TimeProvider interface using the standard time package
type DefaultTimeProvider struct{}

func NewDefaultTimeProvider() interfaces.TimeProvider {
	return &DefaultTimeProvider{}
}

func (p *DefaultTimeProvider) Now() types.Time {
	return types.Time{UnixTime: time.Now().Unix()}
}

func (p *DefaultTimeProvider) Sleep(d types.Duration) {
	time.Sleep(time.Duration(d))
}
