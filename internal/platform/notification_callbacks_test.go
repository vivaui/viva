package platform

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestNotificationRegistryActivationOnceAndCancellation(t *testing.T) {
	var callbacks NotificationRegistry
	var calls atomic.Int32
	id := callbacks.Add(NotificationEvents{Activated: func() { calls.Add(1); callbacks.Remove("other") }})
	var jobs sync.WaitGroup
	for range 32 {
		jobs.Add(1)
		go func() {
			defer jobs.Done()
			if activate := callbacks.Take(id).Activated; activate != nil {
				activate()
			}
		}()
	}
	jobs.Wait()
	if calls.Load() != 1 || callbacks.Has(id) {
		t.Fatal("activation was not consumed exactly once")
	}
	id = callbacks.Add(NotificationEvents{Activated: func() { t.Error("cancelled activation") }})
	callbacks.Remove(id)
	if callbacks.Has(id) || callbacks.Take(id).Activated != nil {
		t.Fatal("cancelled callback retained")
	}
}
