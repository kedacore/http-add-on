package util

import (
	"time"
)

type Stopwatch struct {
	startTime time.Time
	stopTime  time.Time
}

func (sw *Stopwatch) Start() {
	sw.startTime = time.Now()
}

func (sw *Stopwatch) Stop() {
	sw.stopTime = time.Now()
}

func (sw *Stopwatch) StartTime() time.Time {
	return sw.startTime
}

func (sw *Stopwatch) StopTime() time.Time {
	return sw.stopTime
}

func (sw *Stopwatch) ElapsedTime() time.Duration {
	return sw.stopTime.Sub(sw.startTime)
}
