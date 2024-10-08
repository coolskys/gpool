package gpool

import (
	"fmt"
	"gpool/pkg/utils"
	"time"
)

type TaskState uint

const (
	TASK_CREATED TaskState = iota
	TASK_EXECUTING
	TASK_FINISHED
	TASK_FAILED
)

type TaskFn func(args ...interface{}) (interface{}, error) // 非阻塞任务
type TaskBFn func(args ...interface{})                     // 阻塞任务

type task struct {
	id           uint64
	name         string
	stopChan     chan struct{}
	fn           TaskFn
	bfn          TaskBFn
	isBlock      bool
	args         []interface{}
	state        TaskState
	startTime    *time.Time
	finishedTime *time.Time
	result       *Result
}

type TaskStat struct {
	ID           uint64
	State        string
	StartTime    string
	FinishedTime string
}

var (
	sf, _ = utils.NewSnowflake(1)
)

type GTask interface {
	GetTaskID() uint64
	GetArgs() []interface{}
	GetState() string
	Execute(done chan bool)
	Result() *Result
	GetTaskStat() *TaskStat
	Stop()
	IsBlock() bool
}

func NewBlockTask(name string, bfn TaskBFn, args ...interface{}) *task {
	if bfn == nil {
		return nil
	}
	return &task{
		id:      uint64(sf.Generate()),
		name:    name,
		bfn:     bfn,
		args:    args,
		isBlock: true,
	}
}

func NewTask(name string, fn TaskFn, args ...interface{}) GTask {
	if fn == nil {
		return nil
	}
	return &task{
		id:   uint64(sf.Generate()),
		name: name,
		fn:   fn,
		args: args,
	}
}

func (t *task) GetTaskID() uint64 {
	return t.id
}

func (t *task) GetArgs() []interface{} {
	return t.args
}

func (t *task) GetState() string {
	return t.stateString()
}

func (t *task) Result() *Result {
	return t.result
}

func (t *task) IsBlock() bool {
	return t.isBlock
}

func (t *task) Execute(done chan bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("xxxxxxxxxxxxx,", r, t.id)
			t.state = TASK_FAILED
			t.result = &Result{
				Err: fmt.Errorf("%v", r),
			}
			done <- true
		}
	}()

	if t == nil {
		return
	}

	t.startTime = currentTime()
	t.state = TASK_EXECUTING

	if t.isBlock {
		// 阻塞执行
		if t.bfn == nil {
			return
		}
		for {
			select {
			case <-t.stopChan:
				return
			default:
				if t.state != TASK_EXECUTING {
					t.bfn(t.args...)
				}
			}
		}
	}

	// 非阻塞任务直接返回
	var result = &Result{}
	if t.fn == nil {
		return
	}
	data, err := t.fn(t.args...)
	if err != nil {
		t.state = TASK_FAILED
		result.Err = err
		t.result = result
		return
	}

	result.Success = true
	result.Data = data

	t.result = result
	t.state = TASK_FINISHED
	t.finishedTime = currentTime()

	// 执行完成，通知worker
	done <- true
}

func (t *task) Stop() {
	t.stopChan <- struct{}{}
}

func (t *task) stateString() string {
	switch t.state {
	case TASK_CREATED:
		return "created"
	case TASK_EXECUTING:
		return "executing"
	case TASK_FINISHED:
		return "finished"
	case TASK_FAILED:
		return "failed"
	default:
		return ""
	}
}

func (t *task) GetTaskStat() *TaskStat {
	stat := &TaskStat{
		ID:    t.id,
		State: t.stateString(),
	}
	if t.startTime != nil {
		stat.StartTime = t.startTime.Format(time.DateTime)
	}
	if t.finishedTime != nil {
		stat.FinishedTime = t.finishedTime.Format(time.DateTime)
	}
	return stat
}

func currentTime() *time.Time {
	now := time.Now()
	return &now
}
