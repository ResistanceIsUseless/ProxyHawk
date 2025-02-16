package testhelpers

import (
	"fmt"
	"sync"
	"time"
)

// TestResult represents the result of a single test
type TestResult struct {
	Name      string
	Passed    bool
	Message   string
	StartTime time.Time
	EndTime   time.Time
}

// TestProgress tracks the progress of multiple tests
type TestProgress struct {
	mu       sync.Mutex
	results  []TestResult
	current  *TestResult
	started  time.Time
	finished time.Time
}

// NewTestProgress creates a new test progress tracker
func NewTestProgress() *TestProgress {
	return &TestProgress{
		results: make([]TestResult, 0),
		started: time.Now(),
	}
}

// StartTest begins tracking a new test
func (p *TestProgress) StartTest(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = &TestResult{
		Name:      name,
		StartTime: time.Now(),
	}
}

// AddResult adds a test result
func (p *TestProgress) AddResult(result TestResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.current != nil {
		p.current.EndTime = time.Now()
		p.current.Passed = result.Passed
		p.current.Message = result.Message
		p.results = append(p.results, *p.current)
		p.current = nil
	}
}

// PrintSummary prints a summary of all test results
func (p *TestProgress) PrintSummary() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.finished = time.Now()
	totalDuration := p.finished.Sub(p.started)

	fmt.Println("\nTest Results Summary:")
	fmt.Println("====================")

	passed := 0
	failed := 0
	for _, result := range p.results {
		status := "✓"
		if !result.Passed {
			status = "✗"
			failed++
		} else {
			passed++
		}

		duration := result.EndTime.Sub(result.StartTime)
		fmt.Printf("%s %s (%.2fs)\n", status, result.Name, duration.Seconds())
		if !result.Passed && result.Message != "" {
			fmt.Printf("   Error: %s\n", result.Message)
		}
	}

	fmt.Println("\nSummary:")
	fmt.Printf("Total Tests: %d\n", len(p.results))
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total Duration: %.2fs\n", totalDuration.Seconds())
}
