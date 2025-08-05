package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ResistanceIsUseless/ProxyHawk/internal/proxy"
	"github.com/ResistanceIsUseless/ProxyHawk/internal/ui"
)

// Manager handles worker pool management for proxy checking
type Manager struct {
	concurrency int
	debug       bool
	ctx         context.Context
	checker     *proxy.Checker
}

// CheckStatus represents the status of an active check
type CheckStatus struct {
	Proxy      string
	IsActive   bool
	LastUpdate time.Time
}

// ResultHandler is called when a proxy check is completed
type ResultHandler func(result *proxy.ProxyResult)

// UpdateHandler is called when the UI needs to be updated
type UpdateHandler func()

// DebugHandler is called when debug information should be logged
type DebugHandler func(message string)

// NewManager creates a new worker manager
func NewManager(concurrency int, debug bool, ctx context.Context, checker *proxy.Checker) *Manager {
	return &Manager{
		concurrency: concurrency,
		debug:       debug,
		ctx:         ctx,
		checker:     checker,
	}
}

// StartChecking starts the worker pool to check proxies
func (m *Manager) StartChecking(
	proxies []string,
	activeChecks map[string]*ui.CheckStatus,
	mutex *sync.Mutex,
	resultHandler ResultHandler,
	updateHandler UpdateHandler,
	debugHandler DebugHandler,
) {
	var wg sync.WaitGroup
	proxyChan := make(chan string)

	if m.debug && debugHandler != nil {
		debugHandler(fmt.Sprintf("[DEBUG] Starting proxy checks with concurrency: %d", m.concurrency))
		debugHandler(fmt.Sprintf("[DEBUG] Total proxies to check: %d", len(proxies)))
	}

	// Start workers
	for i := 0; i < m.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer func() {
				if r := recover(); r != nil && debugHandler != nil {
					debugHandler(fmt.Sprintf("[ERROR] Worker %d panicked: %v", workerID, r))
				}
				wg.Done()
			}()

			if m.debug && debugHandler != nil {
				debugHandler(fmt.Sprintf("[DEBUG] Worker %d started", workerID))
			}

			for proxy := range proxyChan {
				// Check for cancellation
				select {
				case <-m.ctx.Done():
					if m.debug && debugHandler != nil {
						debugHandler(fmt.Sprintf("[DEBUG] Worker %d cancelled", workerID))
					}
					return
				default:
					// Continue processing
				}

				// Update active status
				if mutex != nil && activeChecks != nil {
					mutex.Lock()
					activeChecks[proxy] = &ui.CheckStatus{
						Proxy:      proxy,
						IsActive:   true,
						LastUpdate: time.Now(),
					}
					mutex.Unlock()
				}

				if updateHandler != nil {
					updateHandler()
				}

				if m.debug && debugHandler != nil {
					debugHandler(fmt.Sprintf("[DEBUG] Worker %d checking: %s", workerID, proxy))
				}

				// Perform the check
				result := m.checker.Check(proxy)

				// Handle the result
				if resultHandler != nil {
					resultHandler(result)
				}

				// Clean up active status
				if mutex != nil && activeChecks != nil {
					mutex.Lock()
					delete(activeChecks, proxy)
					mutex.Unlock()
				}

				if updateHandler != nil {
					updateHandler()
				}
			}
		}(i)
	}

	// Send proxies to workers
	go func() {
		for _, proxy := range proxies {
			select {
			case <-m.ctx.Done():
				close(proxyChan)
				return
			case proxyChan <- proxy:
				// Proxy sent successfully
			}
		}
		close(proxyChan)
	}()

	// Wait for all workers to complete
	wg.Wait()
}
