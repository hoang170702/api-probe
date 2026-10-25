package broker

import (
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

func TestPublishReachesSubscribersOfThatRun(t *testing.T) {
	b := New()
	a, stopA := b.Subscribe("r1")
	other, stopO := b.Subscribe("r2")
	defer stopA()
	defer stopO()
	b.Publish("r1", usecase.Event{Kind: usecase.EventCase})
	if ev := <-a; ev.Kind != usecase.EventCase {
		t.Errorf("got %+v", ev)
	}
	select {
	case ev := <-other:
		t.Errorf("r2 subscriber got %+v", ev)
	default:
	}
}

func TestSlowSubscriberIsDroppedWithoutBlocking(t *testing.T) {
	b := New()
	slow, stop := b.Subscribe("r1") // never reads
	defer stop()
	done := make(chan struct{})
	go func() {
		for range Buffer + 10 {
			b.Publish("r1", usecase.Event{Kind: usecase.EventCase})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow subscriber")
	}
	n := 0
	for range slow { // drains the buffer, then sees the channel closed
		n++
	}
	if n != Buffer {
		t.Errorf("received %d events before the drop, want %d", n, Buffer)
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	b := New()
	ch, stop := b.Subscribe("r1")
	stop()
	stop()
	if _, open := <-ch; open {
		t.Error("channel should be closed")
	}
	b.Publish("r1", usecase.Event{}) // no subscribers left: must not panic
}
