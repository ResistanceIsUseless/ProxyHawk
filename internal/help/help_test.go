package help

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestGetBanner(t *testing.T) {
	// Test with color
	bannerColor := GetBanner(false)
	if !strings.Contains(bannerColor, AppName) {
		t.Errorf("Banner should contain app name")
	}
	if !strings.Contains(bannerColor, Version) {
		t.Errorf("Banner should contain version")
	}
	if !strings.Contains(bannerColor, "\033[") {
		t.Errorf("Colored banner should contain ANSI escape codes")
	}
	
	// Test without color
	bannerNoColor := GetBanner(true)
	if !strings.Contains(bannerNoColor, AppName) {
		t.Errorf("Banner should contain app name")
	}
	if !strings.Contains(bannerNoColor, Version) {
		t.Errorf("Banner should contain version")
	}
	if strings.Contains(bannerNoColor, "\033[") {
		t.Errorf("No-color banner should not contain ANSI escape codes")
	}
}

func TestGetQuickStart(t *testing.T) {
	// Test with color
	quickStartColor := GetQuickStart(false)
	if !strings.Contains(quickStartColor, "QUICK START") {
		t.Errorf("Quick start should contain title")
	}
	if !strings.Contains(quickStartColor, "proxyhawk -l") {
		t.Errorf("Quick start should contain command examples")
	}
	if !strings.Contains(quickStartColor, "\033[") {
		t.Errorf("Colored quick start should contain ANSI escape codes")
	}
	
	// Test without color
	quickStartNoColor := GetQuickStart(true)
	if !strings.Contains(quickStartNoColor, "QUICK START") {
		t.Errorf("Quick start should contain title")
	}
	if !strings.Contains(quickStartNoColor, "proxyhawk -l") {
		t.Errorf("Quick start should contain command examples")
	}
	if strings.Contains(quickStartNoColor, "\033[") {
		t.Errorf("No-color quick start should not contain ANSI escape codes")
	}
}

func TestGetFullHelp(t *testing.T) {
	help := GetFullHelp(true)
	
	// Check for essential sections
	expectedSections := []string{
		"SYNOPSIS",
		"CORE OPTIONS",
		"OUTPUT OPTIONS",
		"PROGRESS OPTIONS",
		"SECURITY & TESTING OPTIONS",
		"ADVANCED OPTIONS",
		"EXAMPLES",
		"SECURITY FEATURES",
		"CONFIGURATION",
		"ENVIRONMENT",
		"MORE INFORMATION",
	}
	
	for _, section := range expectedSections {
		if !strings.Contains(help, section) {
			t.Errorf("Help should contain section: %s", section)
		}
	}
	
	// Check for essential flags
	expectedFlags := []string{
		"-l, --list",
		"-c, --concurrency",
		"-t, --timeout",
		"-v, --verbose",
		"-d, --debug",
		"-o, --output",
		"-j, --json",
		"--no-ui",
		"--progress",
		"--hot-reload",
		"--metrics",
	}
	
	for _, flag := range expectedFlags {
		if !strings.Contains(help, flag) {
			t.Errorf("Help should contain flag: %s", flag)
		}
	}
}

func TestGetExamples(t *testing.T) {
	examples := GetExamples()
	
	if len(examples) == 0 {
		t.Error("Should have at least one example")
	}
	
	// Check that examples have required fields
	for i, ex := range examples {
		if ex.Description == "" {
			t.Errorf("Example %d should have description", i)
		}
		if ex.Command == "" {
			t.Errorf("Example %d should have command", i)
		}
		if !strings.Contains(ex.Command, "proxyhawk") {
			t.Errorf("Example %d command should contain 'proxyhawk'", i)
		}
	}
	
	// Check for specific examples
	commandTexts := make([]string, len(examples))
	for i, ex := range examples {
		commandTexts[i] = ex.Command
	}
	allCommands := strings.Join(commandTexts, " ")
	
	expectedFeatures := []string{
		"-l proxies.txt",
		"-c", "-t",
		"-o", "-j",
		"--no-ui",
		"--rate-limit",
		"--hot-reload",
		"--metrics",
	}
	
	for _, feature := range expectedFeatures {
		if !strings.Contains(allCommands, feature) {
			t.Errorf("Examples should demonstrate feature: %s", feature)
		}
	}
}

func TestPrintHelp(t *testing.T) {
	var buf bytes.Buffer
	
	// Test with color
	PrintHelp(&buf, false)
	helpOutput := buf.String()
	
	if !strings.Contains(helpOutput, AppName) {
		t.Error("Help output should contain app name")
	}
	if !strings.Contains(helpOutput, "SYNOPSIS") {
		t.Error("Help output should contain synopsis")
	}
	
	// Test without color
	buf.Reset()
	PrintHelp(&buf, true)
	helpNoColor := buf.String()
	
	if strings.Contains(helpNoColor, "\033[") {
		t.Error("No-color help should not contain ANSI escape codes")
	}
}

func TestPrintQuickStart(t *testing.T) {
	var buf bytes.Buffer
	
	PrintQuickStart(&buf, true)
	output := buf.String()
	
	if !strings.Contains(output, AppName) {
		t.Error("Quick start output should contain app name")
	}
	if !strings.Contains(output, "QUICK START") {
		t.Error("Quick start output should contain title")
	}
	if !strings.Contains(output, "proxyhawk -l") {
		t.Error("Quick start output should contain command examples")
	}
}

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	
	// Test with color
	PrintVersion(&buf, false)
	versionOutput := buf.String()
	
	if !strings.Contains(versionOutput, AppName) {
		t.Error("Version output should contain app name")
	}
	if !strings.Contains(versionOutput, Version) {
		t.Error("Version output should contain version")
	}
	if !strings.Contains(versionOutput, "github.com") {
		t.Error("Version output should contain repository URL")
	}
	
	// Test without color
	buf.Reset()
	PrintVersion(&buf, true)
	versionNoColor := buf.String()
	
	if strings.Contains(versionNoColor, "\033[") {
		t.Error("No-color version should not contain ANSI escape codes")
	}
}

func TestPrintUsageError(t *testing.T) {
	var buf bytes.Buffer
	testErr := errors.New("test error message")
	
	// Test with color
	PrintUsageError(&buf, testErr, false)
	errorOutput := buf.String()
	
	if !strings.Contains(errorOutput, "test error message") {
		t.Error("Error output should contain error message")
	}
	if !strings.Contains(errorOutput, "Usage:") {
		t.Error("Error output should contain usage information")
	}
	if !strings.Contains(errorOutput, "--help") {
		t.Error("Error output should suggest help flag")
	}
	
	// Test without color
	buf.Reset()
	PrintUsageError(&buf, testErr, true)
	errorNoColor := buf.String()
	
	if strings.Contains(errorNoColor, "\033[") {
		t.Error("No-color error should not contain ANSI escape codes")
	}
}

func TestDetectNoColor(t *testing.T) {
	// Save original environment
	originalProxyHawkNoColor := os.Getenv("PROXYHAWK_NO_COLOR")
	originalNoColor := os.Getenv("NO_COLOR")
	
	defer func() {
		// Restore original environment
		os.Setenv("PROXYHAWK_NO_COLOR", originalProxyHawkNoColor)
		os.Setenv("NO_COLOR", originalNoColor)
	}()
	
	// Test PROXYHAWK_NO_COLOR=1
	os.Setenv("PROXYHAWK_NO_COLOR", "1")
	os.Unsetenv("NO_COLOR")
	if !DetectNoColor() {
		t.Error("Should detect no color when PROXYHAWK_NO_COLOR=1")
	}
	
	// Test NO_COLOR set
	os.Unsetenv("PROXYHAWK_NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	if !DetectNoColor() {
		t.Error("Should detect no color when NO_COLOR is set")
	}
	
	// Test both unset (depends on terminal detection)
	os.Unsetenv("PROXYHAWK_NO_COLOR")
	os.Unsetenv("NO_COLOR")
	// Don't assert the result as it depends on test environment
	DetectNoColor() // Just ensure it doesn't panic
}

func TestColorConstants(t *testing.T) {
	// Ensure color constants are defined
	colors := []string{
		colorReset, colorRed, colorGreen, colorYellow,
		colorBlue, colorPurple, colorCyan, colorBold,
	}
	
	for i, color := range colors {
		if color == "" {
			t.Errorf("Color constant %d should not be empty", i)
		}
	}
}

// Benchmark tests
func BenchmarkGetFullHelp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetFullHelp(true)
	}
}

func BenchmarkGetExamples(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetExamples()
	}
}

func BenchmarkPrintHelp(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		buf.Reset()
		PrintHelp(&buf, true)
	}
}