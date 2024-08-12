package gpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type (
	PoolWorkMode uint8
	PoolState    uint8
)

const (
	DefaultMaxIdleNum       = 2
	DefaultMaxTaskNum       = 1e6
	DefaultCapacity         = 10
	GPoolWorkerNumThreshold = 1e4
	DefaultTimeout          = time.Hour
)

const (
	ModeNoBlock PoolWorkMode = iota
	ModeBlock
)

const (
	StateStopped int64 = iota
	StateStarted
)

// 线程池
type gpool struct {
	mu sync.Mutex

	// waitgroup
	wg sync.WaitGroup

	// 任务队列
	taskQueue chan GTask

	// 工作者列表, 线程池较大时，使用链表结构
	// PushBack()和Front()，PushFront()和Back() - 队列
	// PushBack()和Back(), PushFront()和Front() - 栈
	// workfactory *list.List

	// 工作者列表，线程池较小时，使用数组结构
	// workfactory []*worker

	// 其他工厂
	// otherfactory *list.List

	// 工作中线程数，空闲线程数
	workingNum, idleNum int64

	// 线程池
	workerChan chan *worker

	// 任务结果
	taskResult map[uint64]*Result

	// context
	context context.Context

	// cancel
	cancel context.CancelFunc

	// 状态
	state int64

	options *Options
}

type Pool struct {
	gpool
}

func loadOptions(pool *Pool) {
	if pool.options.capacity <= 0 {
		pool.options.capacity = DefaultCapacity
	}

	if pool.options.maxTaskNum <= 0 {
		pool.options.maxTaskNum = DefaultMaxTaskNum
	}
	if pool.options.timeout <= 0 {
		pool.options.timeout = DefaultTimeout
	}
	pool.workerChan = make(chan *worker, pool.options.capacity)
}

func NewGPool(ctx context.Context, options ...Option) *Pool {
	context, cancel := context.WithCancel(ctx)

	pool := &Pool{gpool: gpool{
		wg:         sync.WaitGroup{},
		mu:         sync.Mutex{},
		context:    context,
		cancel:     cancel,
		taskResult: make(map[uint64]*Result),
		taskQueue:  make(chan GTask, 1),
	}}

	pool.options = &Options{
		mode: ModeBlock,
	}
	for _, opt := range options {
		opt(pool.options)
	}

	loadOptions(pool)
	return pool
}

func (p *Pool) Start() {
	if !atomic.CompareAndSwapInt64(&p.state, StateStopped, StateStarted) {
		return
	}
	// 从taskqueue取出任务执行
	go func() {
		for {
			select {
			case <-p.context.Done():
				fmt.Println("context done 11111111")
				return
			case task := <-p.taskQueue:
				if task != nil {
					w, err := p.Get()
					if err != nil {
						fmt.Println(err)
						return
					}
					if w == nil {
						return
					}
					// 执行任务
					w.task = task
					atomic.AddInt64(&w.pool.workingNum, 1)

					// 对于每个任务，启动两个协程，一个执行任务，一个监听任务
					p.wg.Add(2)
					go w.Execute()
					// 监听任务
					go func() {
						defer p.wg.Done()
						for <-w.done {
							// 存放结果
							fmt.Printf("结束任务：%d\n", w.task.GetTaskID())
							p.mu.Lock()
							w.pool.taskResult[w.task.GetTaskID()] = w.task.Result()
							p.mu.Unlock()

							w.task = nil
							w.sig = WorkerFree

							p.workerChan <- w
							atomic.AddInt64(&p.idleNum, 1)
							atomic.AddInt64(&p.workingNum, -1)
							return
						}
					}()
				}
			}
		}
	}()

	p.wg.Add(1)
	// Debug: 状态输出
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-p.context.Done():
				fmt.Println("context done xxxxx")
				return
			default:
				p.Stat()
				time.Sleep(100 * time.Millisecond)
			}

		}
	}()
}

func (p *Pool) Get() (*worker, error) {
	select {
	case <-p.context.Done():
		return nil, p.context.Err()
	case worker := <-p.workerChan:
		atomic.AddInt64(&p.idleNum, -1)
		return worker, nil
	default:
		if p.isFull() {
			switch p.options.mode {
			case ModeBlock:
				for worker := range p.workerChan {
					if worker != nil {
						atomic.AddInt64(&p.idleNum, -1)
						return worker, nil
					}
				}
			case ModeNoBlock:
				return nil, nil
			default:
				return nil, errors.New("unknown mode")
			}
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		return newWorker(p), nil
	}
}

func (p *Pool) Submit(task GTask) error {
	if p == nil {
		return errors.New("pool not init")
	}

	if task == nil {
		return errors.New("empty task")
	}

	if p.IsClosed() {
		return errors.New("pool is closed")
	}

	select {
	case <-p.context.Done():
		return p.context.Err()
	default:
		p.taskQueue <- task
	}
	return nil
}

// 停止处理任务，释放资源
func (p *Pool) Release() {
	if !atomic.CompareAndSwapInt64(&p.state, StateStarted, StateStopped) {
		return
	}

	fmt.Println("xxxxxxxxxxxx 释放资源")
	p.mu.Lock()
	defer p.mu.Unlock()
	// 停止处理任务
	p.cancel()

	// 等待所有任务处理完成
	p.wg.Wait()

	p.state = StateStopped
	// 关闭通道不再接收任务
	close(p.taskQueue)
	// 关闭通道，不再处理任务
	close(p.workerChan)

	p.taskResult = nil
}

func (p *gpool) isFull() bool {
	return atomic.LoadInt64(&p.options.capacity) <= atomic.LoadInt64(&p.workingNum)
}

func (p *Pool) IsClosed() bool {
	return atomic.LoadInt64(&p.state) == StateStopped
}

func (p *Pool) Stat() {
	fmt.Printf("[%s]线程池大小:%d, 空闲工作线程数: %d, 正在工作线程数:%d\n", time.Now().Format(time.DateTime), p.options.capacity, p.idleNum, p.workingNum)
}

type Result struct {
	Data    interface{}
	Success bool
	Err     error
}

func (p *Pool) GetResult(taskId uint64) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := p.taskResult[taskId]
	return result
}
