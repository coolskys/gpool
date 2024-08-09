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
	fmt.Printf("run with %d\n", n)
}

func demoFuncs(args ...interface{}) {
	time.Sleep(10 * time.Millisecond)
	fmt.Println("Hello World!")
}

func TestGPool(t *testing.T) {
	pool := NewGPool("Test", 100, context.Background())
	pool.Start()

	runTimes := 1000
	var wg sync.WaitGroup
	syncCalculateSum := func(args ...interface{}) (interface{}, error) {
		myFunc(args[0])
		wg.Done()
		return nil, nil
	}
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		var task = NewTask("caculate", syncCalculateSum, int32(i))

		_ = pool.Submit(task)
		fmt.Println("提交任务：", i+1)
	}
	wg.Wait()

	fmt.Println("result:", sum)
	if sum != 499500 {
		panic("caculate error")
	}
	time.Sleep(5 * time.Second)
	pool.Stop()
}
