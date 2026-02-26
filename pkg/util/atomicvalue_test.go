package util

import (
	"math/rand/v2"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AtomicValue", func() {
	var i int

	BeforeEach(func() {
		i = rand.Int()
	})

	Context("New", func() {
		It("returns an AtomicValue with the expected value", func() {
			v := NewAtomicValue(i)

			out, ok := v.atomicValue.Load().(int)
			Expect(ok).To(BeTrue())
			Expect(out).To(Equal(i))
		})
	})

	Context("Get", func() {
		It("returns the stored value", func() {
			var a atomic.Value
			a.Store(i)

			v := AtomicValue[int]{
				atomicValue: a,
			}

			out := v.Get()
			Expect(out).To(Equal(i))
		})
	})

	Context("Set", func() {
		It("stores the expected value", func() {
			var v AtomicValue[int]
			v.Set(i)

			out, ok := v.atomicValue.Load().(int)
			Expect(ok).To(BeTrue())
			Expect(out).To(Equal(i))
		})
	})

	Context("E2E", func() {
		It("succeeds", func() {
			v := NewAtomicValue(i)

			out0 := v.Get()
			Expect(out0).To(Equal(i))

			i = rand.Int()
			v.Set(i)

			out1 := v.Get()
			Expect(out1).To(Equal(i))
		})
	})
})
