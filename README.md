## gpool

### Introduction
A simple goroutine pool implement.

### Main work

When starting the service, first initialize a Goroutine Pool. This Pool maintains a stack-like LIFO queue, which holds Workers responsible for handling tasks. When a client submits a task to the Pool, the core operation inside the Pool after receiving the task is to process it.：

```text
+-----------------------------+
|   Start Service & Initialize Pool   |
+-----------------------------+
                |
                v
+-----------------------------------+
|   Check for Available Workers     |
+-----------------------------------+
                |
        +-------+-------+
        |               |
        v               v
+---------------+  +----------------------+
|  Available    |  |  No Available Workers |
|  Worker Found |  +----------------------+
+---------------+                 |
        |                         v
        v               +------------------------------+
+----------------+      |  Check if Pool is Full        |
|  Execute Task  |      +------------------------------+
+----------------+                 |
                |                 v
                |       +-----------------------------+
                |       |  Is Pool Non-Blocking Mode?  |
                |       +-----------------------------+
                |            |           |
                |            v           v
                |  +------------------+  +--------------------------+
                |  |  Return nil      |  |  Block Until Worker     |
                |  +------------------+  |  Available               |
                |                        +--------------------------+
                |                              |
                v                              v
+----------------+                    +--------------------------+
|  Task Completed |                    |  New Worker (Goroutine)  |
+----------------+                    +--------------------------+
        |                                       |
        v                                       v
+-----------------------------+   +------------------------------+
|  Return Worker to Pool Queue |   |  Execute Task                |
+-----------------------------+   +------------------------------+
```

Example:

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

func main() {
	pool := NewGPool("Default", 100, context.Background())
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
		fmt.Println("commit task：", i+1)
	}
	wg.Wait()

	if sum != 499500 {
		panic("caculate error")
	}
	time.Sleep(5 * time.Second)
	pool.Stop()
}

```