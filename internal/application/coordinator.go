package application

import "sync"

type batchLock struct {
	mu         sync.Mutex
	references int
}

type Coordinator struct {
	mu      sync.Mutex
	locks   map[string]*batchLock
	permits chan struct{}
}

func NewCoordinator(maxParallel int) *Coordinator {
	if maxParallel < 1 {
		maxParallel = 1
	}
	return &Coordinator{locks: map[string]*batchLock{}, permits: make(chan struct{}, maxParallel)}
}

func (c *Coordinator) acquire(batchID string) func() {
	c.permits <- struct{}{}
	c.mu.Lock()
	lock := c.locks[batchID]
	if lock == nil {
		lock = &batchLock{}
		c.locks[batchID] = lock
	}
	lock.references++
	c.mu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		c.mu.Lock()
		lock.references--
		if lock.references == 0 {
			delete(c.locks, batchID)
		}
		c.mu.Unlock()
		<-c.permits
	}
}
