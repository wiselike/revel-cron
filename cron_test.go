package cron

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Many tests schedule a job for every second, and then wait at most a second
// for it to run.  This amount is just slightly larger than 1 second to
// compensate for a few milliseconds of runtime.
const OneSecond = 1*time.Second + 100*time.Millisecond

func errFatal(t testing.TB, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func TestFuncPanicRecovery(t *testing.T) {
	cron := New()
	cron.Start()
	defer cron.Stop()
	errFatal(t, cron.AddFunc("* * * * * ?", func() { panic("YOLO") }))

	select {
	case <-time.After(OneSecond):
		return
	}
}

type DummyJob struct{}

func (d DummyJob) Run() {
	panic("YOLO")
}

func TestJobPanicRecovery(t *testing.T) {
	var job DummyJob

	cron := New()
	cron.Start()
	defer cron.Stop()
	errFatal(t, cron.AddJob("* * * * * ?", job))

	select {
	case <-time.After(OneSecond):
		return
	}
}

// Start and stop cron with no entries.
func TestNoEntries(t *testing.T) {
	cron := New()
	cron.Start()

	select {
	case <-time.After(OneSecond):
		t.Fatal("expected cron will be stopped immediately")
	case <-stop(cron):
	}
}

// Start, stop, then add an entry. Verify entry doesn't run.
func TestStopCausesJobsToNotRun(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.Start()
	cron.Stop()
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))

	select {
	case <-time.After(OneSecond):
		// No job ran!
	case <-wait(wg):
		t.Fatal("expected stopped cron does not run any job")
	}
}

// Add a job, start cron, expect it runs.
func TestAddBeforeRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))
	cron.Start()
	defer cron.Stop()

	// Give cron 2 seconds to run our job (which is always activated).
	select {
	case <-time.After(OneSecond):
		t.Fatal("expected job runs")
	case <-wait(wg):
	}
}

// Start cron, add a job, expect it runs.
func TestAddWhileRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.Start()
	defer cron.Stop()
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))

	select {
	case <-time.After(OneSecond):
		t.Fatal("expected job runs")
	case <-wait(wg):
	}
}

// Test for #34. Adding a job after calling start results in multiple job invocations.
func TestAddWhileRunningWithDelay(t *testing.T) {
	cron := New()
	cron.Start()
	defer cron.Stop()
	time.Sleep(5 * time.Second)
	calls := 0
	errFatal(t, cron.AddFunc("* * * * * *", func() { calls++ }))

	<-time.After(OneSecond)
	if calls != 1 {
		t.Errorf("called %d times, expected 1\n", calls)
	}
}

// Test timing with Entries.
func TestSnapshotEntries(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	errFatal(t, cron.AddFunc("@every 2s", func() { wg.Done() }))
	cron.Start()
	defer cron.Stop()

	// Cron should fire in 2 seconds. After 1 second, call Entries.
	select {
	case <-time.After(OneSecond):
		cron.Entries()
	}

	// Even though Entries was called, the cron should fire at the 2 second mark.
	select {
	case <-time.After(OneSecond):
		t.Error("expected job runs at 2 second mark")
	case <-wait(wg):
	}
}

// Test that the entries are correctly sorted.
// Add a bunch of long-in-the-future entries, and an immediate entry, and ensure
// that the immediate entry runs immediately.
// Also: Test that multiple jobs run in the same instant.
func TestMultipleEntries(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron := New()
	errFatal(t, cron.AddFunc("0 0 0 1 1 ?", func() {}))
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))
	errFatal(t, cron.AddFunc("0 0 0 31 12 ?", func() {}))
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(OneSecond):
		t.Error("expected job run in proper order")
	case <-wait(wg):
	}
}

// Test running the same job twice.
func TestRunningJobTwice(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron := New()
	errFatal(t, cron.AddFunc("0 0 0 1 1 ?", func() {}))
	errFatal(t, cron.AddFunc("0 0 0 31 12 ?", func() {}))
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(2 * OneSecond):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

func TestRunningMultipleSchedules(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron := New()
	errFatal(t, cron.AddFunc("0 0 0 1 1 ?", func() {}))
	errFatal(t, cron.AddFunc("0 0 0 31 12 ?", func() {}))
	errFatal(t, cron.AddFunc("* * * * * ?", func() { wg.Done() }))
	cron.Schedule(Every(time.Minute), FuncJob(func() {}))
	cron.Schedule(Every(time.Second), FuncJob(func() { wg.Done() }))
	cron.Schedule(Every(time.Hour), FuncJob(func() {}))

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(2 * OneSecond):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

// Test that the cron is run in the local time zone (as opposed to UTC).
func TestLocalTimezone(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	now := time.Now()
	spec := fmt.Sprintf("%d,%d %d %d %d %d ?",
		(now.Second()+1)%60, (now.Second()+2)%60, now.Minute(),
		now.Hour(), now.Day(), now.Month())

	cron := New()
	errFatal(t, cron.AddFunc(spec, func() { wg.Done() }))
	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(OneSecond * 2):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

// Test that the cron is run in the given time zone (as opposed to local).
func TestNonLocalTimezone(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	loc, err := time.LoadLocation("Atlantic/Cape_Verde")
	if err != nil {
		t.Errorf("Failed to load time zone Atlantic/Cape_Verde: %+v",
			err)
	}

	now := time.Now().In(loc)
	spec := fmt.Sprintf("%d,%d %d %d %d %d ?",
		(now.Second()+1)%60, (now.Second()+2)%60, now.Minute(),
		now.Hour(), now.Day(), now.Month())

	cron := NewWithLocation(loc)
	errFatal(t, cron.AddFunc(spec, func() { wg.Done() }))
	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(OneSecond * 2):
		t.Error("expected job fires 2 times")
	case <-wait(wg):
	}
}

// Test that calling stop before start silently returns without
// blocking the stop channel.
func TestStopWithoutStart(t *testing.T) {
	cron := New()
	cron.Stop()
}

type testJob struct {
	wg   *sync.WaitGroup
	name string
}

func (t testJob) Run() {
	t.wg.Done()
}

// Simple test using Runnables.
func TestJob(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	errFatal(t, cron.AddJob("0 0 0 30 Feb ?", testJob{wg, "job0"}))
	errFatal(t, cron.AddJob("0 0 0 1 1 ?", testJob{wg, "job1"}))
	errFatal(t, cron.AddJob("* * * * * ?", testJob{wg, "job2"}))
	errFatal(t, cron.AddJob("1 0 0 1 1 ?", testJob{wg, "job3"}))
	cron.Schedule(Every(5*time.Second+5*time.Nanosecond), testJob{wg, "job4"})
	cron.Schedule(Every(5*time.Minute), testJob{wg, "job5"})

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(OneSecond):
		t.FailNow()
	case <-wait(wg):
	}

	// Ensure the entries are in the right order.
	expecteds := []string{"job2", "job4", "job5", "job1", "job3", "job0"}

	var actuals []string
	for _, entry := range cron.Entries() {
		actuals = append(actuals, entry.Job.(testJob).name)
	}

	for i, expected := range expecteds {
		if actuals[i] != expected {
			t.Fatalf("Jobs not in the right order.  (expected) %s != %s (actual)", expecteds, actuals)
		}
	}
}

type ZeroSchedule struct{}

func (*ZeroSchedule) Next(time.Time) time.Time {
	return time.Time{}
}

// Tests that job without time does not run.
func TestJobWithZeroTimeDoesNotRun(t *testing.T) {
	cron := New()
	calls := 0
	errFatal(t, cron.AddFunc("* * * * * *", func() { calls++ }))
	cron.Schedule(new(ZeroSchedule), FuncJob(func() {
		t.Error("expected zero task will not run")
	}))
	cron.Start()
	defer cron.Stop()
	<-time.After(OneSecond)
	if calls != 1 {
		t.Errorf("called %d times, expected 1\n", calls)
	}
}

func wait(wg *sync.WaitGroup) chan bool {
	ch := make(chan bool)
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}

func stop(cron *Cron) chan bool {
	ch := make(chan bool)
	go func() {
		cron.Stop()
		ch <- true
	}()
	return ch
}
