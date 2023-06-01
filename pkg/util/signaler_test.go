package util

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signaler", func() {
	Context("New", func() {
		It("returns a signaler of capacity 1", func() {
			i := NewSignaler()

			s, ok := i.(signaler)
			Expect(ok).To(BeTrue())

			c := cap(s)
			Expect(c).To(Equal(1))
		})
	})

	Context("Signal", func() {
		It("produces on channel", func() {
			s := make(signaler, 1)

			err := WithTimeout(time.Second, DeapplyError(s.Signal, nil))
			Expect(err).NotTo(HaveOccurred())

			select {
			case <-s:
			default:
				Fail("channel should not be empty")
			}
		})

		It("does not block when channel is full", func() {
			s := make(signaler, 1)
			s <- struct{}{}

			err := WithTimeout(time.Second, DeapplyError(s.Signal, nil))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Wait", func() {
		It("returns nil when channel is not empty", func() {
			ctx := context.TODO()

			s := make(signaler, 1)
			s <- struct{}{}

			err := WithTimeout(time.Second, ApplyContext(s.Wait, ctx))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns err when context is done", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			cancel()

			s := make(signaler, 1)

			err := WithTimeout(time.Second, ApplyContext(s.Wait, ctx))
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	Context("E2E", func() {
		It("succeeds", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			s := NewSignaler()

			err0 := WithTimeout(time.Second, DeapplyError(s.Signal, nil))
			Expect(err0).NotTo(HaveOccurred())

			err1 := WithTimeout(time.Second, DeapplyError(s.Signal, nil))
			Expect(err1).NotTo(HaveOccurred())

			err2 := WithTimeout(time.Second, ApplyContext(s.Wait, ctx))
			Expect(err2).NotTo(HaveOccurred())

			cancel()

			err3 := WithTimeout(time.Second, ApplyContext(s.Wait, ctx))
			Expect(err3).To(MatchError(context.Canceled))
		})
	})
})
