package queue

import (
	"sync"
)

type PriorityQueue struct {
	items []*Job
	mu    sync.RWMutex
}

func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		items: make([]*Job, 0),
	}
}

func (pq *PriorityQueue) Push(job *Job) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Insert in priority order (higher priority first)
	inserted := false
	for i, item := range pq.items {
		if job.Priority > item.Priority {
			// Insert at position i
			pq.items = append(pq.items[:i], append([]*Job{job}, pq.items[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		// Add to end if lowest priority
		pq.items = append(pq.items, job)
	}
}

func (pq *PriorityQueue) Pop() *Job {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return nil
	}

	job := pq.items[0]
	pq.items = pq.items[1:]

	return job
}

func (pq *PriorityQueue) Peek() *Job {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if len(pq.items) == 0 {
		return nil
	}

	return pq.items[0]
}

func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Len() == 0
}

func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items = make([]*Job, 0)
}

func (pq *PriorityQueue) Remove(jobID string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i, job := range pq.items {
		if job.ID == jobID {
			// Remove the job
			pq.items = append(pq.items[:i], pq.items[i+1:]...)
			return true
		}
	}
	return false
}

func (pq *PriorityQueue) GetAll() []*Job {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*Job, len(pq.items))
	copy(result, pq.items)
	return result
}

func (pq *PriorityQueue) GetByType(jobType JobType) []*Job {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make([]*Job, 0)
	for _, job := range pq.items {
		if job.Type == jobType {
			result = append(result, job)
		}
	}
	return result
}

func (pq *PriorityQueue) GetByPriority(priority int) []*Job {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make([]*Job, 0)
	for _, job := range pq.items {
		if job.Priority == priority {
			result = append(result, job)
		}
	}
	return result
}