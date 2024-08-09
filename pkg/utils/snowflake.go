package utils

import (
	"fmt"
	"sync"
	"time"
)

/**
  雪花id 使用的一些条件限制
    1. 运行timestamp_mask(ms)后生成的id会重复
    2. 同一个workid 生成的id会重复
    3. 序号位数越多 并发时产生的等待时间会越少
    4. 系统时间不能回拨，回拨时间会导致id重复（经常掉电的设备上不适合使用）
*/

const (
	// 2010-02-01 06:28:00
	timestamp_start  = int64(1265005680000)                            //开始时间戳 :ms
	workid_bits_n    = int64(9)                                        // 机器id所占用位数
	sequence_bits_n  = int64(12)                                       // 序列所占位数
	timestamp_bits_n = int64(64 - sequence_bits_n - workid_bits_n - 1) // 最高位留空(这样不会变成负数)

	workerid_mask   = int64(-1 ^ (-1 << workid_bits_n))    //最大机器id 8：0~255, 9:0~511
	sequence_mask   = int64(-1 ^ (-1 << sequence_bits_n))  //最大序列数
	timestamp_mask  = int64(-1 ^ (-1 << timestamp_bits_n)) // 时间戳最大值:ms  0000FFFFFFFFFFFF
	workid_shift    = sequence_bits_n                      //机器id左移位数
	timestamp_shift = sequence_bits_n + workid_bits_n
)

type SnowFlake struct {
	sync.Mutex
	timestamp int64 // 当前时间(ms)
	workerId  int64
	sequence  int64 // 序号
}

func NewSnowflake(workerid int64) (*SnowFlake, error) {
	if workerid < 0 || workerid > workerid_mask {
		return nil, fmt.Errorf("worker id must be between 0 and %d", workerid_mask)
	}
	return &SnowFlake{
		timestamp: 0,
		workerId:  workerid,
		sequence:  0,
	}, nil
}

func (s *SnowFlake) Generate() int64 {
	s.Lock()
	now := time.Now().UnixMilli()
	//now := time.Now().UnixNano() / 1000000
	if s.timestamp == now {
		s.sequence = (s.sequence + 1) & sequence_mask
		if s.sequence == 0 { // 用完序号
			for now <= s.timestamp { // 等待到下一毫秒
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}
	s.timestamp = now
	r := int64((now-timestamp_start)<<timestamp_shift | (s.workerId << workid_shift) | (s.sequence))
	s.Unlock()
	return r
}
