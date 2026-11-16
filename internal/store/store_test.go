package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ManuelLuchage/apura/internal/core"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "sub", "apura.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)

	s, err := st.CreateSession(ctx, core.Session{Name: "k3s", Goal: "cluster", Tags: []string{"K3s, hetzner", "k3s"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSession(ctx, core.Session{Name: "k3s"}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("want ErrNameTaken, got %v", err)
	}

	for i, c := range []struct {
		cmd   string
		exit  int
		techs []string
	}{
		{"apt install k3s", 100, []string{"apt"}},
		{"sudo apt install k3s", 0, []string{"apt"}},
		{"kubectl get nodes", 0, []string{"kubernetes"}},
	} {
		got, err := st.AddStep(ctx, core.Step{SessionID: s.ID, Command: c.cmd, ExitCode: c.exit, Techs: c.techs})
		if err != nil {
			t.Fatal(err)
		}
		if got.Seq != i+1 {
			t.Fatalf("seq = %d, want %d", got.Seq, i+1)
		}
	}

	if n, err := st.SetMark(ctx, s.ID, []int{1}, core.MarkFail); err != nil || n != 1 {
		t.Fatalf("SetMark: %d %v", n, err)
	}
	last, _ := st.LastStep(ctx, s.ID)
	if _, err := st.AddNote(ctx, s.ID, &last.ID, "node Ready after 30s"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddNote(ctx, s.ID, nil, "session note"); err != nil {
		t.Fatal(err)
	}

	steps, err := st.Steps(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Mark != core.MarkFail || len(steps[2].Notes) != 1 || steps[2].Techs[0] != "kubernetes" {
		t.Fatalf("unexpected steps: %+v", steps)
	}

	got, err := st.Resolve(ctx, "k3s")
	if err != nil {
		t.Fatal(err)
	}
	if got.StepCount != 3 || len(got.Tags) != 2 || len(got.Techs) != 2 {
		t.Fatalf("aggregates: %+v", got)
	}

	// filters
	for name, f := range map[string]ListFilter{
		"tag":   {Tag: "hetzner"},
		"tech":  {Tech: "kubernetes"},
		"query": {Query: "Ready after"},
		"cmd":   {Query: "kubectl get"},
		"state": {State: core.StateActive},
	} {
		list, err := st.ListSessions(ctx, f)
		if err != nil || len(list) != 1 {
			t.Errorf("filter %s: %d results, err %v", name, len(list), err)
		}
	}
	if list, _ := st.ListSessions(ctx, ListFilter{Tech: "docker"}); len(list) != 0 {
		t.Error("tech filter should not match")
	}

	if n, _ := st.DeleteSteps(ctx, s.ID, []int{1}); n != 1 {
		t.Fatal("delete")
	}
	if err := st.EndSession(ctx, s.ID, core.ResultSuccess); err != nil {
		t.Fatal(err)
	}
	got, _ = st.Session(ctx, s.ID)
	if got.State != core.StateEnded || got.Result != core.ResultSuccess || got.EndedAt == nil || got.StepCount != 2 {
		t.Fatalf("after end: %+v", got)
	}
	// new steps continue after the highest seq, not filling gaps
	nst, _ := st.AddStep(ctx, core.Step{SessionID: s.ID, Command: "x"})
	if nst.Seq != 4 {
		t.Fatalf("seq after delete = %d, want 4", nst.Seq)
	}
}

func TestConcurrentWritersAcrossConnections(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "apura.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := st.CreateSession(ctx, core.Session{Name: "c"})
	st.Close()

	// Simulate several shell hooks writing at once (separate processes → separate DB handles).
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, err := Open(path)
			if err != nil {
				errs <- err
				return
			}
			defer h.Close()
			if _, err := h.AddStep(ctx, core.Step{SessionID: s.ID, Command: "echo"}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	h, _ := Open(path)
	defer h.Close()
	steps, _ := h.Steps(ctx, s.ID)
	if len(steps) != 20 || steps[19].Seq != 20 {
		t.Fatalf("got %d steps, last seq %d", len(steps), steps[len(steps)-1].Seq)
	}
}
