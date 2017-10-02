package rbp_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/reedobrien/rbp"
)

func TestBufferCacheDefaults(t *testing.T) {
	tut := rbp.NewBufferPool()

	b := tut.Get()
	equals(t, len(b), 32*1024)
	equals(t, tut.Lifetime, time.Minute)
}

func TestBufferCache(t *testing.T) {
	tut := rbp.NewBufferPool(
		func(bp *rbp.BufferPool) {
			bp.Lifetime = 40 * time.Millisecond
		},
		func(bp *rbp.BufferPool) {
			bp.BufferSize = 1
		})

	var ba [][]byte
	for i := 100; i > 0; i-- {
		ba = append(ba, tut.Get())
	}

	got := tut.Stats()
	equals(t, got.Created >= 100, true)
	equals(t, got.Created <= 101, true)
	equals(t, got.Freed, 0)
	equals(t, got.PoolSize <= 1, true)

	for _, b := range ba {
		tut.Put(b)
	}

	got = tut.Stats()
	equals(t, got.Created >= 100 && got.Created <= 101, true)
	equals(t, got.Freed, 0)
	equals(t, got.PoolSize > 99, true)

	time.Sleep(50 * time.Millisecond)
	got = tut.Stats()
	equals(t, got.Created >= 100, true)
	equals(t, got.Freed > 0, true)
	equals(t, got.PoolSize < 99, true)

	b := tut.Get()
	equals(t, len(b), 1)
}

// Thanks benbjohnson
// equals fails the test if got is not equal to want.
func equals(tb testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\tgot: %#v\n\n\twant: %#v\033[39m\n\n", filepath.Base(file), line, got, want)
		tb.FailNow()
	}
}
