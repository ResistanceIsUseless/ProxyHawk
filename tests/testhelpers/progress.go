package testhelpers

import (
	"fmt"
	"strings"
)

// TestProgress tracks test progress and results
type TestProgress struct {
	totalTests   int
	passedTests  int
	failedTests  int
	skippedTests int
	currentTest  string
	testResults  []TestResult
}

// TestResult represents a single test result
type TestResult struct {
	Name    string
	Passed  bool
	Skipped bool
	Message string
	Details []string
}

// NewTestProgress creates a new test progress tracker
func NewTestProgress() *TestProgress {
	return &TestProgress{}
}

// StartTest starts a new test with the given name
func (tp *TestProgress) StartTest(name string) {
	tp.currentTest = name
	fmt.Printf("\n%s %s\n", infoStyle.Render("Running:"), name)
}

// AddResult adds a test result
func (tp *TestProgress) AddResult(result TestResult) {
	tp.testResults = append(tp.testResults, result)
	if result.Skipped {
		tp.skippedTests++
	} else if result.Passed {
		tp.passedTests++
	} else {
		tp.failedTests++
	}

	// Print immediate result
	status := successStyle.Render("✓ PASS")
	if result.Skipped {
		status = warningStyle.Render("⚠ SKIP")
	} else if !result.Passed {
		status = errorStyle.Render("✗ FAIL")
	}

	fmt.Printf("  %s %s\n", status, result.Name)
	if result.Message != "" {
		fmt.Printf("    %s\n", detailStyle.Render(result.Message))
	}
	for _, detail := range result.Details {
		fmt.Printf("      %s\n", detailStyle.Render(detail))
	}
}

// PrintSummary prints the test summary
func (tp *TestProgress) PrintSummary() {
	total := tp.passedTests + tp.failedTests + tp.skippedTests
	fmt.Printf("\n%s\n", strings.Repeat("─", 50))
	fmt.Printf("%s\n", infoStyle.Render("Test Summary:"))
	fmt.Printf("Total Tests: %d\n", total)
	fmt.Printf("  %s: %d\n", successStyle.Render("Passed"), tp.passedTests)
	fmt.Printf("  %s: %d\n", errorStyle.Render("Failed"), tp.failedTests)
	fmt.Printf("  %s: %d\n", warningStyle.Render("Skipped"), tp.skippedTests)
	fmt.Printf("%s\n", strings.Repeat("─", 50))

	if tp.failedTests > 0 {
		fmt.Printf("\n%s\n", errorStyle.Render("Failed Tests:"))
		for _, result := range tp.testResults {
			if !result.Passed && !result.Skipped {
				fmt.Printf("✗ %s\n", result.Name)
				if result.Message != "" {
					fmt.Printf("  %s\n", result.Message)
				}
				for _, detail := range result.Details {
					fmt.Printf("    %s\n", detail)
				}
			}
		}
	}
}
