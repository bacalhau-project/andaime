package common_interface

//go:generate mockery --name Clienter --output ../../../../mocks/common --outpkg common
// Clienter defines the common interface for cloud clients
type Clienter interface{}
