package utils

import (
	"os"
	"reflect"
	"sync"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
)

var GlobalChannels []interface {
	Close()
	IsClosed() bool
	SetClosed()
}
var GlobalChannelsMutex sync.Mutex

type SafeChannel struct {
	Ch     interface{}
	Closed bool
	Mu     sync.Mutex
}

func NewSafeChannel(ch interface{}) *SafeChannel {
	return &SafeChannel{
		Ch:     ch,
		Closed: false,
		Mu:     sync.Mutex{},
	}
}

func (sc *SafeChannel) Close() {
	sc.Mu.Lock()
	defer sc.Mu.Unlock()
	if !sc.Closed {
		defer func() {
			if r := recover(); r != nil {
				// Channel was already closed, just log it
				logger.Get().Warnf("Attempted to close an already closed channel: %v", sc.Ch)
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

// CreateAndRegisterChannel creates a new channel of the specified type and capacity, and registers it
func CreateAndRegisterChannel(channelType reflect.Type, capacity int) interface{} {
	ch := reflect.MakeChan(channelType, capacity).Interface()
	RegisterChannel(ch)
	return ch
}

func RegisterChannel(ch interface{}) {
	safeChannel := NewSafeChannel(ch)
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	GlobalChannels = append(GlobalChannels, safeChannel)
}

func CloseAllChannels() {
	l := logger.Get()
	l.Debugf("Closing all channels")
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for i, ch := range GlobalChannels {
		if ch != nil {
			if !ch.IsClosed() {
				l.Debugf("Closing channel %v", ch)
				ch.Close()
			} else {
				l.Debugf("Channel %v is already closed", ch)
			}
		}
		GlobalChannels[i] = nil
	}
	GlobalChannels = nil
}

// Helper functions for common channel types
func CreateSignalChannel(capacity int) chan os.Signal {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan os.Signal)(nil)).Elem(), capacity).(chan os.Signal)
}

func CreateStructChannel(capacity int) chan struct{} {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan struct{})(nil)).Elem(), capacity).(chan struct{})
}

func CreateErrorChannel(capacity int) chan error {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan error)(nil)).Elem(), capacity).(chan error)
}

func CreateStringChannel(capacity int) chan string {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan string)(nil)).Elem(), capacity).(chan string)
}

func CreateEventChannel(capacity int) chan events.EngineEvent {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan events.EngineEvent)(nil)).Elem(),
		capacity).(chan events.EngineEvent)
}

func CreateBoolChannel(capacity int) chan bool {
	return CreateAndRegisterChannel(reflect.TypeOf((*chan bool)(nil)).Elem(), capacity).(chan bool)
}

// CloseChannel closes a specific channel
func CloseChannel(ch interface{}) {
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for _, safeChannel := range GlobalChannels {
		if reflect.ValueOf(safeChannel).Pointer() == reflect.ValueOf(ch).Pointer() {
			if safeChannel.IsClosed() {
				continue
			}
			safeChannel.Close()
			return
		}
	}
	// If the channel wasn't found, it might not have been registered
	l := logger.Get()
	l.Warnf("Attempted to close an unregistered channel: %v", ch)
}

func IsChannelClosed(ch interface{}) bool {
	GlobalChannelsMutex.Lock()
	defer GlobalChannelsMutex.Unlock()
	for _, safeChannel := range GlobalChannels {
		if reflect.ValueOf(safeChannel).Pointer() == reflect.ValueOf(ch).Pointer() {
			return safeChannel.IsClosed()
		}
	}
	return true
}
