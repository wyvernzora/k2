package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
)

const defaultJobOutputTailLines = 8

// JobSpec is one unit inside a JobGroup. Run receives an output writer
// that captures diagnostics for failure summaries without interleaving
// concurrent job output into the terminal.
type JobSpec struct {
	Name string
	Run  func(ctx context.Context, out io.Writer) error
}

// CommandJob builds a JobSpec backed by a non-interactive subprocess.
func CommandJob(name string, spec CommandSpec) JobSpec {
	return JobSpec{
		Name: name,
		Run: func(ctx context.Context, out io.Writer) error {
			cmd := spec.command(ctx)
			cmd.Stdout = out
			cmd.Stderr = out
			cmd.Stdin = spec.Stdin
			return cmd.Run()
		},
	}
}

// JobGroupOptions controls JobGroup scheduling and failure behavior.
type JobGroupOptions struct {
	Concurrency     int
	FailFast        bool
	OutputTailLines int
}

// JobResult is the stable result record for one job.
type JobResult struct {
	Name     string
	Err      error
	Output   string
	Warnings []string
	Duration time.Duration
}

type queuedJob struct {
	index int
	spec  JobSpec
}

func runJobGroup(ctx context.Context, jobs []JobSpec, opts JobGroupOptions) ([]JobResult, error) {
	concurrency, tailLines := normalizeJobGroupOptions(len(jobs), opts)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	queue := make(chan queuedJob)
	normalizedJobs, results := initializeJobResults(jobs)
	waitForWorkers := startJobWorkers(runCtx, cancel, queue, results, opts.FailFast, tailLines, concurrency)
	enqueueJobs(runCtx, queue, normalizedJobs)
	waitForWorkers()

	return results, collectJobGroupErrors(results, normalizedJobs)
}

func normalizeJobGroupOptions(jobCount int, opts JobGroupOptions) (concurrency int, tailLines int) {
	concurrency = opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > jobCount {
		concurrency = jobCount
	}
	tailLines = opts.OutputTailLines
	if tailLines <= 0 {
		tailLines = defaultJobOutputTailLines
	}
	return concurrency, tailLines
}

func initializeJobResults(jobs []JobSpec) ([]JobSpec, []JobResult) {
	normalizedJobs := make([]JobSpec, len(jobs))
	results := make([]JobResult, len(jobs))
	for i, job := range jobs {
		if job.Name == "" {
			job.Name = fmt.Sprintf("job-%d", i+1)
		}
		normalizedJobs[i] = job
		results[i] = JobResult{Name: job.Name, Err: context.Canceled}
	}
	return normalizedJobs, results
}

func startJobWorkers(
	ctx context.Context,
	cancel context.CancelFunc,
	queue <-chan queuedJob,
	results []JobResult,
	failFast bool,
	tailLines int,
	concurrency int,
) func() {
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range queue {
				results[job.index] = runOneJob(ctx, job.spec, tailLines)
				if failFast && results[job.index].Err != nil {
					cancel()
				}
			}
		}()
	}
	return wg.Wait
}

func enqueueJobs(ctx context.Context, queue chan<- queuedJob, jobs []JobSpec) {
enqueue:
	for i, job := range jobs {
		select {
		case <-ctx.Done():
			break enqueue
		case queue <- queuedJob{index: i, spec: job}:
		}
	}
	close(queue)
}

func collectJobGroupErrors(results []JobResult, normalizedJobs []JobSpec) error {
	var errs []error
	for i, result := range results {
		if result.Name == "" {
			results[i].Name = normalizedJobs[i].Name
		}
		if result.Err != nil {
			if result.Output != "" {
				errs = append(errs, fmt.Errorf("%s: %w\n%s", result.Name, result.Err, result.Output))
			} else {
				errs = append(errs, fmt.Errorf("%s: %w", result.Name, result.Err))
			}
		}
	}
	return errors.Join(errs...)
}

func runOneJob(ctx context.Context, spec JobSpec, tailLines int) JobResult {
	start := time.Now()
	out := newLineTail(tailLines)
	err := ctx.Err()
	if err == nil {
		if spec.Run == nil {
			err = fmt.Errorf("no job function")
		} else {
			err = spec.Run(ctx, out)
		}
	}
	return JobResult{
		Name:     spec.Name,
		Err:      err,
		Output:   out.String(),
		Warnings: out.Warnings(),
		Duration: time.Since(start).Round(time.Millisecond),
	}
}

type lineTail struct {
	mu       sync.Mutex
	maxLines int
	pending  bytes.Buffer
	lines    []string
	warnings []string
}

func newLineTail(maxLines int) *lineTail {
	return &lineTail{maxLines: maxLines}
}

func (t *lineTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, b := range p {
		if b == '\n' || b == '\r' {
			t.flushLocked()
			continue
		}
		_ = t.pending.WriteByte(b)
	}
	return len(p), nil
}

func (t *lineTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flushLocked()
	return strings.Join(t.lines, "\n")
}

func (t *lineTail) Warnings() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flushLocked()
	return append([]string(nil), t.warnings...)
}

func (t *lineTail) flushLocked() {
	line := strings.TrimSpace(t.pending.String())
	t.pending.Reset()
	if line == "" {
		return
	}
	if strings.HasPrefix(line, "warning:") {
		t.warnings = append(t.warnings, line)
	}
	t.lines = append(t.lines, line)
	if len(t.lines) > t.maxLines {
		t.lines = t.lines[len(t.lines)-t.maxLines:]
	}
}
