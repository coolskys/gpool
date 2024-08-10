package gpool

import (
	"context"
	"fmt"
	"sync"
)

type workerState uint

const (
	WorkerFree workerState = iota
	WorkerBinded
	WorkerWorking
)

// 工作者
type worker struct {
	pool   *Pool
	task   ITask
	ctx    context.Context
	cancel context.CancelFunc

	done chan bool

	mu  sync.Mutex
	sig workerState
}

type IWorker interface {
	Execute(ctx context.Context)
	Kill(err error)
	State() string
}

func newWorker(pool *Pool) *worker {
	ctx, cancel := context.WithCancel(pool.context)
	worker := &worker{
		mu:     sync.Mutex{},
		pool:   pool,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan bool),
	}
	return worker
}

// 执行任务
func (w *worker) Execute(ctx context.Context) {
	defer w.pool.wg.Done()

	w.mu.Lock()
	w.sig = WorkerWorking
	w.mu.Unlock()

	if w.task.IsBlock() {
		for {
			select {
			case <-w.ctx.Done():
				// 监听到worker退出，则停止任务执行
				fmt.Println("context done")
				w.task.Stop()
				return
			case <-w.done:
				return
			default:
				w.pool.once.Do(func() {
					w.task.Execute(w.done)
				})
			}
		}
	} else {
		w.pool.once.Do(func() {
			w.task.Execute(w.done)
		})
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
