package file

import (
	"context"
	"sync"
)

type ExplorerConstructor func(ctx context.Context) (map[string]Explorer, error)

type LazyDevice struct {
	mu          sync.RWMutex
	constructor ExplorerConstructor
	devices     map[string]Explorer
	initError   error
}

func NewLazyDevice(constructor ExplorerConstructor) *LazyDevice {
	return &LazyDevice{
		constructor: constructor,
	}
}

func (l *LazyDevice) Get(ctx context.Context) (map[string]Explorer, error) {
	l.mu.RLock()
	if l.devices != nil {
		l.mu.RUnlock()
		return l.devices, nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.devices != nil {
		return l.devices, nil
	}

	devices, err := l.constructor(ctx)
	if err != nil {
		l.initError = err
		return nil, err
	}

	l.devices = devices
	return devices, nil
}

func (l *LazyDevice) InitError() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.initError
}
