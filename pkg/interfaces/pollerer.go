package interfaces

import "context"

type Pollerer interface {
	Poll(ctx context.Context) error
}
