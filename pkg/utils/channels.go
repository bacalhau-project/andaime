package utils

import (
	"os"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/pulumi/pulumi-azure-native-sdk/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
)

var GlobalChannels []interface {
	Close()
	IsClosed() bool
	SetClosed()
	GetName() string
	GetChannel() interface{}
}
var GlobalChannelsMutex sync.Mutex

type SafeChannel struct {
	Ch     interface{}
	Closed bool
	Mu     sync.Mutex
	Name   string
}

func NewSafeChannel(ch interface{}, name string) *SafeChannel {
	return &SafeChannel{
		Ch:     ch,
		Closed: false,
		Mu:     sync.Mutex{},
		Name:   name,
	}
}

func (sc *SafeChannel) Close() {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	if !sc.Closed {
		defer func() {
			if r := recover(); r != nil {
				// Channel was already closed, just log it
				logger.Get().Warnf("Attempted to close an already closed channel: %s", sc.Name)
			}
		}()
		reflect.ValueOf(sc.Ch).Close()
		sc.Closed = true
	}
}

func (sc *SafeChannel) IsClosed() bool {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	return sc.Closed
}

func (sc *SafeChannel) SetClosed() {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	sc.Closed = true
}

func (sc *SafeChannel) GetName() string {
	return sc.Name
}

func (sc *SafeChannel) GetChannel() interface{} {
	return sc.Ch
}

// CreateAndRegisterChannel creates a new channel of the specified type and capacity, and registers it
func CreateAndRegisterChannel(channelType reflect.Type, name string, capacity int) interface{} {
	ch := reflect.MakeChan(channelType, capacity).Interface()
	safeChannel := NewSafeChannel(ch, name)
	RegisterChannel(safeChannel, name)
	return ch
}

func RegisterChannel(ch interface{}) {
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	if safeChannel, ok := ch.(*SafeChannel); ok {
		GlobalChannels = append(GlobalChannels, safeChannel)
	}
}

func CloseAllChannels() {
	l := logger.Get()
	l.Debugf("Closing all channels - via events.CloseAllChannels")
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for _, ch := range GlobalChannels {
		if ch != nil {
			if !ch.IsClosed() {
				l.Debugf("Closing channel %v", ch.GetName())
				ch.Close()
			} else {
				l.Debugf("Channel %v is already closed", ch.GetName())
			}
		}
		ch.SetClosed()
	}
}

// Helper functions for common channel types
func CreateSignalChannel(name string, capacity int) chan os.Signal {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan os.Signal)(nil)).Elem(),
		name, capacity).(chan os.Signal)
}

func CreateStructChannel(name string, capacity int) chan struct{} {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan struct{})(nil)).Elem(),
		name, capacity).(chan struct{})
}

func CreateErrorChannel(name string, capacity int) chan error {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan error)(nil)).Elem(),
		name, capacity).(chan error)
}

func CreateStringChannel(name string, capacity int) chan string {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan string)(nil)).Elem(),
		name, capacity).(chan string)
}

func CreateEventChannel(name string, capacity int) chan events.EngineEvent {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan events.EngineEvent)(nil)).Elem(),
		name, capacity).(chan events.EngineEvent)
}

func CreateBoolChannel(name string, capacity int) chan bool {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan bool)(nil)).Elem(),
		name, capacity).(chan bool)
}

func CreateVMChannel(name string, capacity int) chan *compute.VirtualMachine {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan *compute.VirtualMachine)(nil)).Elem(),
		name, capacity).(chan *compute.VirtualMachine)
}

func CreateMachineChannel(name string, capacity int) chan *models.Machine {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan *models.Machine)(nil)).Elem(),
		name, capacity).(chan *models.Machine)
}

// CloseChannel closes a specific channel
func CloseChannel(ch interface{}) {
	l := logger.Get()

	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()

	channelPtr := reflect.ValueOf(ch).Pointer()
	l.Debugf("Attempting to close channel with pointer: %v", channelPtr)

	for i, safeChannel := range GlobalChannels {
		safeChannelPtr := reflect.ValueOf(safeChannel.GetChannel()).Pointer()
		l.Debugf("Comparing with registered channel %d: %v", i, safeChannelPtr)

		if safeChannelPtr == channelPtr {
			if safeChannel.IsClosed() {
				l.Debugf("Channel %v is already closed", safeChannel.GetName())
				return
			}
			l.Debugf("Closing channel %v", safeChannel.GetName())
			// safeChannel.Close()
			return
		}
	}

	// If the channel wasn't found, log more details
	stackTrace := debug.Stack()
	l.Warnf(
		"Attempted to close an unregistered channel: %v (type: %T)\nStack trace:\n%s",
		ch,
		ch,
		stackTrace,
	)
}

func IsChannelClosed(chName string) bool {
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for _, safeChannel := range GlobalChannels {
		if safeChannel.GetName() == chName {
			return safeChannel.IsClosed()
		}
	}
	return true
}

func DebugOpenChannels() {
	l := logger.Get()
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for i, ch := range GlobalChannels {
		if ch != nil && !ch.IsClosed() {
			l.Debugf("Open channel %d: %v", i, ch)
		}
	}
}

func AreAllChannelsClosed() bool {
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for _, ch := range GlobalChannels {
		if !ch.IsClosed() {
			return false
		}
	}
	return true
}
