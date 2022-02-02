// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

type Cleaner interface {
	Cleanup()
	AddCleanupFn(f func())
}

func NewCleaner() Cleaner {
	return &cleaner{}
}

type cleaner struct {
	sigNotifier sync.Once
	mu          sync.RWMutex
	cleanups    []func()
}

func (c *cleaner) setupSignalHandling() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
		c.doCleanup()
		p, _ := os.FindProcess(os.Getpid())
		if err := p.Signal(os.Interrupt); err != nil {
			panic(err)
		}
	}()
}

func (c *cleaner) Cleanup() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.doCleanup()
}

func (c *cleaner) doCleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range c.cleanups {
		f()
	}
}

func (c *cleaner) AddCleanupFn(f func()) {
	c.sigNotifier.Do(c.setupSignalHandling)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanups = append(c.cleanups, f)
}
