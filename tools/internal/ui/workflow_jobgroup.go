package ui

import (
	"context"
	"errors"
	"strings"
)

func (w *Workflow) JobGroup(label string, jobsFn func() []JobSpec, opts JobGroupOptions) *StepHandle {
	return w.add("jobs "+label, func(ctx context.Context) error {
		jobs := jobsFn()
		if len(jobs) == 0 {
			w.reporter.Warnf("%s: no jobs", label)
			return nil
		}
		w.reporter.Infof("%s: running %d jobs", label, len(jobs))
		results, err := runJobGroup(ctx, jobs, opts)
		rows := make([][]string, 0, len(results))
		for _, result := range results {
			status := "ok"
			if errors.Is(result.Err, context.Canceled) {
				status = "canceled"
			} else if result.Err != nil {
				status = "failed"
			}
			rows = append(rows, []string{result.Name, status, result.Duration.String()})
		}
		w.reporter.Table([]string{"Job", "Status", "Duration"}, rows)
		w.reportJobWarnings(results)
		if err != nil {
			w.reporter.Errorf("%s failed: %v", label, err)
			return err
		}
		w.reporter.Successf("%s completed", label)
		return nil
	})
}

func (w *Workflow) reportJobWarnings(results []JobResult) {
	for _, result := range results {
		if result.Err != nil || len(result.Warnings) == 0 {
			continue
		}
		for _, line := range result.Warnings {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if warning, ok := strings.CutPrefix(line, "warning:"); ok {
				w.reporter.Warnf("%s: %s", result.Name, strings.TrimSpace(warning))
			}
		}
	}
}
