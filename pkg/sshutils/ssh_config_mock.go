package sshutils

// CommandBehavior defines expected command behavior
type CommandBehavior struct {
	Output string
	Error  error
}

// ServiceBehavior defines expected service behavior
type ServiceBehavior struct {
	Name    string
	Content string
	Output  string
	Error   error
}
