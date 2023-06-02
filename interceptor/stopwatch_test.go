package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stopwatch", func() {
	Context("Start", func() {
		It("records current time on startTime", func() {
			var sw Stopwatch

			sw.Start()
			Expect(sw.startTime).To(BeTemporally("~", time.Now(), time.Millisecond))
		})
	})

	Context("Stop", func() {
		It("records current time on stopTime", func() {
			var sw Stopwatch

			sw.Stop()
			Expect(sw.stopTime).To(BeTemporally("~", time.Now(), time.Millisecond))
		})
	})

	Context("StartTime", func() {
		It("returns the expected value", func() {
			var (
				st = time.Now().Add(-time.Minute)
			)

			sw := Stopwatch{
				startTime: st,
			}

			ret := sw.StartTime()
			Expect(ret).To(Equal(st))
		})
	})

	Context("StopTime", func() {
		It("returns the expected value", func() {
			var (
				st = time.Now().Add(+time.Minute)
			)

			sw := Stopwatch{
				stopTime: st,
			}

			ret := sw.StopTime()
			Expect(ret).To(Equal(st))
		})
	})

	Context("ElapsedTime", func() {
		It("returns the difference between startTime and stopTime", func() {
			var (
				at = time.Now().Add(-time.Minute)
				ot = time.Now().Add(+time.Minute)
				du = ot.Sub(at)
			)

			sw := &Stopwatch{
				startTime: at,
				stopTime:  ot,
			}

			ret := sw.ElapsedTime()
			Expect(ret).To(Equal(du))
		})
	})
})
