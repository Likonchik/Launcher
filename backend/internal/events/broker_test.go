package events

import (
	"testing"
	"time"
)

func TestBrokerDeliversToSubscribers(t *testing.T) {
	broker := NewBroker()
	_, chA := broker.Subscribe()
	_, chB := broker.Subscribe()

	broker.Publish("profiles")

	for i, ch := range []<-chan string{chA, chB} {
		select {
		case msg := <-ch:
			if msg != "profiles" {
				t.Fatalf("subscriber %d got %q, want \"profiles\"", i, msg)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive event", i)
		}
	}
}

func TestBrokerUnsubscribeStopsDelivery(t *testing.T) {
	broker := NewBroker()
	id, ch := broker.Subscribe()
	broker.Unsubscribe(id)

	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after Unsubscribe")
	}

	// Publish после отписки не должен паниковать.
	broker.Publish("profiles")
}

func TestBrokerPublishDoesNotBlockOnFullBuffer(t *testing.T) {
	broker := NewBroker()
	broker.Subscribe() // не читаем канал

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			broker.Publish("profiles")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber buffer")
	}
}
