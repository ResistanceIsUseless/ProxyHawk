package tests

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup test environment
	setupTestEnvironment()

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestEnvironment()

	os.Exit(code)
}

func setupTestEnvironment() {
	// Create test directories if needed
	os.MkdirAll("test_output", 0755)
}

func cleanupTestEnvironment() {
	// Remove test directories and files
	os.RemoveAll("test_output")
	os.Remove("test_output.txt")
	os.Remove("test_output.json")
	os.Remove("test_working.txt")
	os.Remove("test_working_anonymous.txt")
}
