package gpool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type workerState uint

var (
	id int64 = 0
)

const (
	WorkerFree workerState = iota
	WorkerBinded
	WorkerWorking
)

// 工作者
type worker struct {
	id     int64
	pool   *Pool
	task   GTask
	ctx    context.Context
	cancel context.CancelFunc

	done chan bool

	mu  sync.Mutex
	sig workerState
}

type IWorker interface {
	Execute()
	Kill(err error)
	State() string
}

func newWorker(pool *Pool) *worker {
	ctx, cancel := context.WithCancel(pool.context)
	worker := &worker{
		id:     atomic.AddInt64(&id, 1),
		mu:     sync.Mutex{},
		pool:   pool,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan bool),
	}
	return worker
}

// 执行任务
func (w *worker) Execute() {
	defer w.pool.wg.Done()

	w.mu.Lock()
	w.sig = WorkerWorking
	w.mu.Unlock()

	// 一般是定时任务
	if w.task.IsBlock() {
		for {
			select {
			// 监听到进程退出
			case <-w.ctx.Done():
				w.task.Stop()
				return
			case <-w.done:
				// 任务执行完成
				return
			default:
				w.task.Execute(w.done)
			}
		}
	} else {
		fmt.Printf("workerID:%d,  taskID:%d\n", w.id, w.task.GetTaskID())
		w.task.Execute(w.done)
	}
}

// 结束任务
func (w *worker) Kill(err error) {
	// 取消任务执行
	if err != nil {
		if w.task != nil {
			w.task.Stop()
		}
	}
	w.cancel()

	w.mu.Lock()
	w.sig = WorkerFree
	w.task = nil
	w.mu.Unlock()
}

func (w *worker) State() string {
	switch w.sig {
	case WorkerFree:
		return "free"
	case WorkerBinded:
		return "binded"
	case WorkerWorking:
		return "working"
	default:
		return ""
	}
}
