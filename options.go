package gpool

import "time"

type Option func(*Options)

type Options struct {
	// 名称
	name string
	// 容量
	capacity int64
	// 任务执行超时时间
	timeout time.Duration

	maxTaskNum int64

	// 工作模式
	mode PoolWorkMode
}

func WithOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

func WithName(name string) Option {
	return func(opts *Options) {
		opts.name = name
	}
}

func WithCapacity(capacity int64) Option {
	return func(opts *Options) {
		opts.capacity = capacity
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.timeout = timeout
	}
}

func WithMode(mode PoolWorkMode) Option {
	return func(opts *Options) {
		opts.mode = mode
	}
}
