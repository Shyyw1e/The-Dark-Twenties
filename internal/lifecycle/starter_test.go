package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeService struct {
	name      string
	startErr  error
	stopErr   error
	started   bool
	stopped   bool
	stopOrder *[]string
}

func (s *fakeService) Name() string {
	return s.name
}

func (s *fakeService) Start(ctx context.Context) error {
	if s.startErr != nil {
		return s.startErr
	}
	s.started = true
	return nil
}

func (s *fakeService) Stop(ctx context.Context) error {
	s.stopped = true
	if s.stopOrder != nil {
		*s.stopOrder = append(*s.stopOrder, s.name)
	}
	return s.stopErr
}

func TestStarterStartAndStop(t *testing.T) {
	var order []string
	first := &fakeService{name: "first", stopOrder: &order}
	second := &fakeService{name: "second", stopOrder: &order}
	starter := NewStarter(nil, time.Second)
	starter.Add(first)
	starter.Add(second)

	if err := starter.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !first.started || !second.started {
		t.Fatalf("services not started: first=%v second=%v", first.started, second.started)
	}

	if err := starter.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if got := order; len(got) != 2 || got[0] != "second" || got[1] != "first" {
		t.Fatalf("stop order = %v, want [second first]", got)
	}
}

func TestStarterStopsStartedServicesWhenStartFails(t *testing.T) {
	wantErr := errors.New("boom")
	first := &fakeService{name: "first"}
	second := &fakeService{name: "second", startErr: wantErr}
	starter := NewStarter(nil, time.Second)
	starter.Add(first)
	starter.Add(second)

	err := starter.Start(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if !first.stopped {
		t.Fatal("first service was not stopped after second failed")
	}
	if second.stopped {
		t.Fatal("failed service should not be stopped as started")
	}
}
