package util

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KeyedBroadcaster", func() {
	Context("Broadcast", func() {
		It("wakes every subscriber for the key", func() {
			b := NewKeyedBroadcaster[string]()

			channels := make([]<-chan struct{}, 3)
			for i := range channels {
				var unsubscribe func()
				channels[i], unsubscribe = b.Subscribe("route")
				DeferCleanup(unsubscribe)
			}

			b.Broadcast("route")

			for _, ch := range channels {
				Eventually(ch).Should(Receive())
			}
		})

		It("does not wake subscribers for another key", func() {
			b := NewKeyedBroadcaster[string]()
			ch, unsubscribe := b.Subscribe("route-a")
			DeferCleanup(unsubscribe)

			b.Broadcast("route-b")

			Consistently(ch, 100*time.Millisecond).ShouldNot(Receive())
		})

		It("coalesces broadcasts until the subscriber receives", func() {
			b := NewKeyedBroadcaster[string]()
			ch, unsubscribe := b.Subscribe("route")
			DeferCleanup(unsubscribe)

			b.Broadcast("route")
			b.Broadcast("route")

			Eventually(ch).Should(Receive())
			Consistently(ch, 100*time.Millisecond).ShouldNot(Receive())
		})

		It("does not wake an unsubscribed channel", func() {
			b := NewKeyedBroadcaster[string]()
			ch, unsubscribe := b.Subscribe("route")
			unsubscribe()
			unsubscribe()

			b.Broadcast("route")

			Consistently(ch, 100*time.Millisecond).ShouldNot(Receive())
		})

		It("does not block without subscribers", func() {
			b := NewKeyedBroadcaster[string]()

			b.Broadcast("route")
			b.Broadcast("route")
		})
	})
})
