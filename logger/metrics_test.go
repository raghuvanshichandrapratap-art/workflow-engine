package logger

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounters_ConcurrentIncrements(t *testing.T) {
	c := NewCounters()

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				c.ExecutionsStarted.Add(1)
				c.ExecutionsSucceeded.Add(1)
			}
		}()
	}
	wg.Wait()

	snap := c.Snapshot()
	assert.Equal(t, int64(goroutines*perGoroutine), snap.ExecutionsStarted)
	assert.Equal(t, int64(goroutines*perGoroutine), snap.ExecutionsSucceeded)
	assert.Equal(t, int64(0), snap.ExecutionsFailed)
	assert.Equal(t, int64(0), snap.QueueErrors)
}

func TestCounters_IsolatedBetweenInstances(t *testing.T) {
	a := NewCounters()
	b := NewCounters()

	a.ExecutionsFailed.Add(5)

	assert.Equal(t, int64(5), a.Snapshot().ExecutionsFailed)
	assert.Equal(t, int64(0), b.Snapshot().ExecutionsFailed)
}
