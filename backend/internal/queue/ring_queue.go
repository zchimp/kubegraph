package queue

import "sync"

// RingDropOldQueue 环形缓冲队列
// 规则：达到容量上限时，自动丢弃队首最旧的数据，存入新数据
type RingDropOldQueue[T any] struct {
	mu       sync.Mutex
	capacity int
	buf      []T
	read     int // 读指针
	write    int // 写指针
	size     int // 当前元素数量
}

// NewRingDropOldQueue 创建队列
func NewRingDropOldQueue[T any](capacity int) *RingDropOldQueue[T] {
	if capacity <= 0 {
		capacity = 500
	}
	return &RingDropOldQueue[T]{
		capacity: capacity,
		buf:      make([]T, capacity),
	}
}

// Push 放入元素；如果已满，删除最老一条，再写入新元素
func (q *RingDropOldQueue[T]) Push(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 队列已满：淘汰最旧消息
	if q.size == q.capacity {
		q.read = (q.read + 1) % q.capacity
		q.size--
	}

	q.buf[q.write] = item
	q.write = (q.write + 1) % q.capacity
	q.size++
}

// Pop 获取一条数据，无数据返回 (zero,false)
func (q *RingDropOldQueue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.size == 0 {
		var zero T
		return zero, false
	}

	val := q.buf[q.read]
	q.read = (q.read + 1) % q.capacity
	q.size--
	return val, true
}

// Len 当前队列堆积数量（用于监控水位）
func (q *RingDropOldQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// Capacity 返回最大容量
func (q *RingDropOldQueue[T]) Capacity() int {
	return q.capacity
}
