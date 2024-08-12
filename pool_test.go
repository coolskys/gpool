package gpool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var sum int32

func myFunc(i interface{}) {
	n := i.(int32)
	atomic.AddInt32(&sum, n)
	fmt.Println(1 / sum)
}

func TestGPool(t *testing.T) {
	pool := NewGPool(context.Background(),
		WithName("default"),
		WithCapacity(10),
		WithMode(ModeBlock),
		WithTimeout(30*time.Second),
	)
	pool.Start()
	defer pool.Release()

	runTimes := 20
	var wg = sync.WaitGroup{}
	syncCalculateSum := func(args ...interface{}) (interface{}, error) {
		myFunc(args[0])
		wg.Done()
		return nil, nil
	}
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		var task = NewTask("caculate", syncCalculateSum, int32(i))

		_ = pool.Submit(task)
	}
	wg.Wait()
	fmt.Println("result:", sum)
	if sum != 20*19/2 {
		t.Errorf("caculate error")
		return
	}
}
