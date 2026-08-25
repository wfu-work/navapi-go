package services

import "time"

const (
	backgroundQueueSize = 128
	backgroundWorkerNum = 2
)

var backgroundQueue = make(chan func(), backgroundQueueSize)

func init() {
	for range backgroundWorkerNum {
		go runBackgroundWorker()
	}
}

func runBackgroundWorker() {
	for task := range backgroundQueue {
		if task == nil {
			continue
		}
		task()
	}
}

// enqueueBackground gives fire-and-forget notifications a bounded queue. A
// short wait absorbs ordinary bursts without creating an unbounded goroutine
// backlog; if the queue stays full, the notification is intentionally dropped
// rather than taking down request-serving goroutines.
func enqueueBackground(task func()) bool {
	if task == nil {
		return false
	}
	select {
	case backgroundQueue <- task:
		return true
	default:
	}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case backgroundQueue <- task:
		return true
	case <-timer.C:
		return false
	}
}
