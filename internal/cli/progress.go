package cli

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// ProgressReporter reports progress of the reconnaissance pipeline.
type ProgressReporter struct {
	mu     sync.Mutex
	quiet  bool
	start  time.Time
	prefix string
}

// NewProgressReporter creates a new ProgressReporter.
func NewProgressReporter(quiet bool) *ProgressReporter {
	return &ProgressReporter{
		quiet:  quiet,
		start:  time.Now(),
		prefix: Bold(Cyan("reposcout:")),
	}
}

// Start reports the start of a phase.
func (p *ProgressReporter) Start(phase string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s %s...\n", p.prefix, phase)
}

// Startf reports the start of a phase with formatted message.
func (p *ProgressReporter) Startf(format string, args ...interface{}) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s...\n", p.prefix, msg)
}

// Done reports completion of a phase.
func (p *ProgressReporter) Done() {
	if p.quiet {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := time.Since(p.start).Round(time.Millisecond)
	fmt.Fprintf(os.Stderr, "%s done %s\n", p.prefix, Gray(elapsed.String()))
}

// DoneWithCount reports completion with a count.
func (p *ProgressReporter) DoneWithCount(count int, label string) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := time.Since(p.start).Round(time.Millisecond)
	fmt.Fprintf(os.Stderr, "%s done (%d %s) %s\n", p.prefix, count, label, Gray(elapsed.String()))
}

// Infof reports an informational message.
func (p *ProgressReporter) Infof(format string, args ...interface{}) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s %s\n", p.prefix, fmt.Sprintf(format, args...))
}

// Error reports an error.
func (p *ProgressReporter) Error(err error) {
	if p.quiet {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s %s %v\n", p.prefix, Red("error:"), err)
}

// Phase represents a pipeline phase.
type Phase string

const (
	PhaseScanning   Phase = "scanning repository"
	PhaseExpanding  Phase = "expanding candidates"
	PhaseBuilding   Phase = "building file cards"
	PhaseRanking    Phase = "ranking candidates"
	PhaseAssembling Phase = "building context pack"
	PhaseLLMRerank  Phase = "running LLM rerank"
)

// ReportPhase reports progress for a pipeline phase.
func (p *ProgressReporter) ReportPhase(phase Phase) {
	p.Start(string(phase))
}
