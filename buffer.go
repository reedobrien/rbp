package rbp

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// NewBufferPool constructs and starts a BufferPool
func NewBufferPool(options ...func(*BufferPool)) *BufferPool {
	get := make(chan []byte)
	put := make(chan []byte)

	size := 32 * 1024
	lifetime := time.Minute
	q := new(list.List)
	bp := &BufferPool{
		get:        get,
		put:        put,
		BufferSize: size,
		Lifetime:   lifetime,
		q:          q,
	}

	for _, opt := range options {
		opt(bp)

	}

	bp.start()

	return bp
}

// queued is a container for limiting lifetimes of pooled []byte slices.
type queued struct {
	when  time.Time
	slice []byte
}

// BufferPool implements httputil.BufferPool.
type BufferPool struct {
	BufferSize int
	Lifetime   time.Duration

	mu      sync.RWMutex
	created int
	freed   int
	q       *list.List

	get chan []byte
	put chan []byte
}

// Get returns a []byte slice from the pool.
func (bp *BufferPool) Get() []byte {
	return <-bp.get
}

// Put returns a []byte slice to the pool.
func (bp *BufferPool) Put(b []byte) {
	bp.put <- b
}

// stat holds BufferPool statistics.
type stat struct {
	Created  int
	Freed    int
	PoolSize int
}

// String implements fmt.Stringer.
func (s stat) String() string {
	return fmt.Sprintf("created: %d\nfreed: %d\nPool Size: %d\n", s.Created, s.Freed, s.PoolSize)
}

// Stats returns BufferPool statistics.
func (bp *BufferPool) Stats() stat {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return stat{Created: bp.created, Freed: bp.freed, PoolSize: bp.q.Len()}
}

// makeBuffer creates a 32K []byte slice.
func (bp *BufferPool) makeBuffer() []byte {
	bp.created++
	return make([]byte, bp.BufferSize)
}

// start launches a recycling/expiring routine.
func (bp *BufferPool) start() {
	go func() {
		for {
			if bp.q.Len() == 0 {
				bp.mu.Lock()
				bp.q.PushFront(queued{
					when:  time.Now(),
					slice: bp.makeBuffer(),
				})
				bp.mu.Unlock()
			}
			bp.mu.Lock()
			e := bp.q.Front()
			bp.mu.Unlock()

			timeout := time.NewTimer(bp.Lifetime)
			select {
			case b := <-bp.put:
				timeout.Stop()
				bp.mu.Lock()
				bp.q.PushFront(queued{when: time.Now(), slice: b})
				bp.mu.Unlock()
			case bp.get <- e.Value.(queued).slice:
				timeout.Stop()
				bp.mu.Lock()
				bp.q.Remove(e)
				bp.mu.Unlock()
			case <-timeout.C:
				e := bp.q.Front()
				for e != nil {
					n := e.Next()
					if time.Since(e.Value.(queued).when) > bp.Lifetime {
						bp.mu.Lock()
						bp.q.Remove(e)
						bp.mu.Unlock()
						e.Value = nil
						bp.freed++
					}
					e = n
				}

			}
		}
	}()
}
