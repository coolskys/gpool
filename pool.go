package gpool

import (
	"container/list"
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
	DefaultMaxIdleNum = 2

	DefaultMaxTaskNum = 10000000

	// 线程池数量达到1000000时，使用链表结构来构建线程池
	GPoolWorkerNumThreshold = 10000000
)

const (
	DefaultTimeout = time.Hour
)

const (
	ModeNoBlock PoolWorkMode = iota
	ModeBlock
)

const (
	StateStopped PoolState = iota
	StateStarted
)

// 线程池
type gpool struct {
	// 名称
	name string

	// 任务队列
	taskQueue chan ITask

	mu sync.Mutex

	// 工作者列表, 线程池较大时，使用链表结构
	// PushBack()和Front()，PushFront()和Back() - 队列
	// PushBack()和Back(), PushFront()和Front() - 栈
	workfactory *list.List

	// 工作者列表，线程池较小时，使用数组结构
	// workfactory []*worker

	// 其他工厂
	otherfactory *list.List

	// 工作者数
	workingNum int64

	workerChan chan *worker

	// 容量
	capacity int64

	// idleRelated
	// idleWorkerChan chan *worker
	idleNum int64
	// maxIdleNum int64
	taskResult map[uint64]*Result

	// context
	context context.Context

	// cancel
	cancel context.CancelFunc

	// waitgroup
	wg sync.WaitGroup

	// 停止信号
	stopedChan chan bool

	// 任务执行超时时间
	timeout time.Duration

	// 状态
	state PoolState

	// 工作模式
	mode PoolWorkMode
}

type IPool interface {
	// 启动
	Start()

	Submit(task ITask) error

	// 停止
	Stop() <-chan bool

	// 获取任务执行结果
	GetResult(taskId uint64) *Result
}

func NewGPool(name string, poolSize int64, ctx context.Context) IPool {
	context, cancel := context.WithCancel(ctx)
	workerNum := poolSize
	pool := &gpool{
		wg:           sync.WaitGroup{},
		name:         name,
		capacity:     workerNum,
		workfactory:  list.New(),
		otherfactory: list.New(),
		context:      context,
		cancel:       cancel,
		mode:         ModeBlock,
		taskResult:   make(map[uint64]*Result),
		timeout:      DefaultTimeout,
		taskQueue:    make(chan ITask, DefaultMaxTaskNum),
		workerChan:   make(chan *worker, poolSize),
		stopedChan:   make(chan bool, 1),
	}
	return pool
}

func (p *gpool) SetMode(mode PoolWorkMode) {
	p.mode = mode
}

func (p *gpool) Start() {
	// 避免重复启动
	if p.state == StateStarted {
		return
	}

	// 从taskqueue取出任务执行
	go func() {
		for {
			select {
			case <-p.context.Done():
				return
			case <-p.stopedChan:
				return
			case task := <-p.taskQueue:
				if task != nil {
					w, err := p.Get()
					if err != nil || w == nil {
						return
					}
					// 执行任务
					w.task = task
					atomic.AddInt64(&w.pool.workingNum, 1)

					p.wg.Add(2)
					go w.Execute(p.context)
					// 监听任务
					go func() {
						defer p.wg.Done()
						for <-w.done {
							// 存放结果
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

	// Debug: 状态输出
	go func() {
		for {
			select {
			case <-p.context.Done():
				return
			case <-p.stopedChan:
				return
			default:
				p.Stat()
				time.Sleep(100 * time.Millisecond)
			}

		}
	}()
	go p.wg.Wait()
}

func (p *gpool) Get() (*worker, error) {
	select {
	case <-p.context.Done():
		return nil, p.context.Err()
	case <-p.stopedChan:
		return nil, errors.New("stopped")
	case worker := <-p.workerChan:
		atomic.AddInt64(&p.idleNum, -1)
		return worker, nil
	default:
		if p.isFull() {
			switch p.mode {
			case ModeBlock:
				// 阻塞等待可用的worker:
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

func (p *gpool) Submit(task ITask) error {
	if p == nil {
		return errors.New("pool not init")
	}

	if task == nil {
		return errors.New("empty task")
	}

	select {
	case <-p.context.Done():
		return p.context.Err()
	case <-p.stopedChan:
		return errors.New("stopped")
	default:
		go func() { p.taskQueue <- task }()
	}
	return nil
}

func (p *gpool) Stop() <-chan bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancel()

	p.state = StateStopped
	close(p.workerChan)
	close(p.taskQueue)
	p.taskResult = nil

	return p.stopedChan
}

func (p *gpool) isFull() bool {
	return p.capacity <= p.workingNum
}

func (p *gpool) Stat() {
	fmt.Printf("[%s]线程池大小:%d, 空闲工作线程数: %d, 正在工作线程数:%d\n", time.Now().Format(time.DateTime), p.capacity, p.idleNum, p.workingNum)
}

type Result struct {
	Data    interface{}
	Success bool
	Err     error
}

func (p *gpool) GetResult(taskId uint64) *Result {
	result := p.taskResult[taskId]
	return result
}
