# ProxyHawk TUI Review & Recommendations

## Executive Summary

**Overall Grade: B+ (Good, but room for improvement)**

The ProxyHawk TUI implementation follows most Bubble Tea best practices and has a clean component-based architecture. However, there are several critical issues that need addressing:

### Critical Issues 🔴
1. **Goroutine in Init()** - Major anti-pattern
2. **Manual ticker management** - Error-prone and causes UI freezing
3. **Missing WindowSizeMsg handling** - Will break on terminal resize
4. **Direct goroutine communication** - Violates Elm architecture

### Moderate Issues 🟡
5. **Complex Update() logic** - Heavy computation in update loop
6. **Mutex in Model** - Sign of shared state issues
7. **No

 proper component isolation** - Parent directly accesses child state

### Minor Issues 🟢
8. **View method conditional logic** - Could be simplified
9. **Missing graceful shutdown** - Context cancellation not integrated with tea.Quit
10. **No tests** - Pure functions should be easily testable

---

## Detailed Analysis

### 1. 🔴 CRITICAL: Goroutine in Init()

**Location:** [cmd/proxyhawk/main.go:530-536](cmd/proxyhawk/main.go#L530-L536)

```go
func (s *AppState) Init() tea.Cmd {
    // ❌ ANTI-PATTERN: Starting goroutine in Init
    go s.startChecking()

    return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
        return tickMsg{}
    })
}
```

**Problem:**
- Violates Bubble Tea's Elm architecture
- Goroutine started outside tea.Cmd system
- No way for Bubble Tea to track or manage it
- Cannot be tested easily
- Causes race conditions

**Impact:** High - This is the root cause of many other issues

**Correct Pattern:**
```go
func (s *AppState) Init() tea.Cmd {
    // ✅ Return a command that starts checking
    return tea.Batch(
        startCheckingCmd(s),  // Wraps goroutine in tea.Cmd
        tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
            return tickMsg{}
        }),
    )
}

// Wrap goroutine work in a tea.Cmd
func startCheckingCmd(s *AppState) tea.Cmd {
    return func() tea.Msg {
        // Start workers - this runs in its own goroutine managed by Bubble Tea
        results := s.startChecking()
        return checkingCompleteMsg{results: results}
    }
}
```

---

### 2. 🔴 CRITICAL: Manual Ticker Management

**Location:** [cmd/proxyhawk/main.go:539-606](cmd/proxyhawk/main.go#L539-L606)

```go
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg, progressUpdateMsg:
        // ... lots of logic ...

        // ❌ ANTI-PATTERN: Manually creating new ticker every update
        return s, tea.Batch(
            progressCmd,
            tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
                return tickMsg{}
            }),
        )
    }

    // ❌ CRITICAL BUG: Also returning ticker here
    return s, tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
        return tickMsg{}
    })
}
```

**Problems:**
1. Creates ticker in three different places
2. Ticker continues even when not needed
3. Caused the UI freezing issue we fixed earlier
4. No way to stop/pause updates
5. Ticker runs at same speed regardless of mode

**Impact:** High - Causes UI freezing, wastes CPU

**Correct Pattern:**
```go
// Use bubbles/timer for proper ticker management
import "github.com/charmbracelet/bubbles/timer"

type AppState struct {
    ticker timer.Model
    // ...
}

func initialModel() AppState {
    return AppState{
        ticker: timer.NewWithInterval(100*time.Millisecond, 100*time.Millisecond),
        // ...
    }
}

func (s AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Update ticker
    var cmd tea.Cmd
    s.ticker, cmd = s.ticker.Update(msg)
    cmds = append(cmds, cmd)

    switch msg := msg.(type) {
    case timer.TickMsg:
        // Handle tick - ticker automatically continues
        s.updateMetrics()

    case checkCompleteMsg:
        s.handleCheckComplete(msg)
    }

    return s, tea.Batch(cmds...)
}
```

---

### 3. 🔴 CRITICAL: No WindowSizeMsg Handling

**Location:** [cmd/proxyhawk/main.go:539](cmd/proxyhawk/main.go#L539)

```go
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // ...
    case tickMsg, progressUpdateMsg:
        // ...
    // ❌ MISSING: No case for tea.WindowSizeMsg
    }
}
```

**Problem:**
- When terminal is resized, layout breaks
- Progress bars don't adjust width
- Components may overflow or get cut off
- Guaranteed issue on every terminal resize

**Impact:** High - Poor user experience, looks broken

**Solution:**
```go
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        s.width = msg.Width
        s.height = msg.Height

        // Update progress bar width
        s.view.Progress.Width = msg.Width - 4 // Account for padding

        // Propagate to child components if needed
        return s, nil

    // ... other cases
    }
}
```

---

### 4. 🔴 CRITICAL: Direct Goroutine Communication

**Location:** [cmd/proxyhawk/main.go:624](cmd/proxyhawk/main.go#L624) and throughout `startChecking()`

```go
func (s *AppState) startChecking() {
    // ... goroutine setup ...

    // ❌ ANTI-PATTERN: Direct channel send from goroutine
    s.updateChan <- progressUpdateMsg{}

    // ❌ ANTI-PATTERN: Direct mutex access from goroutine
    s.mutex.Lock()
    s.view.AddDebugMessage("...")
    s.mutex.Unlock()
}
```

**Problems:**
1. Worker goroutines directly modify shared state
2. Requires mutex to prevent races
3. Uses custom channel (`updateChan`) instead of Bubble Tea messages
4. Channel forwarding goroutine (lines 470-475) adds complexity
5. No way to track message flow or test interactions

**Impact:** High - Race conditions, hard to debug, violates Elm architecture

**Correct Pattern:**
```go
// Worker sends results via tea.Cmd
func checkProxyCmd(proxy string, checker *proxy.Checker) tea.Cmd {
    return func() tea.Msg {
        result := checker.Check(proxy)
        return proxyCheckedMsg{
            proxy:  proxy,
            result: result,
        }
    }
}

// Main model handles all state updates
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case proxyCheckedMsg:
        // Update state atomically in Update()
        s.results = append(s.results, msg.result)
        s.view.Current++
        s.view.Working++

        // No mutex needed - Update() is single-threaded
        return s, nil
    }
}
```

---

### 5. 🟡 MODERATE: Heavy Computation in Update()

**Location:** [cmd/proxyhawk/main.go:559-579](cmd/proxyhawk/main.go#L559-L579)

```go
case tickMsg, progressUpdateMsg:
    // ⚠️ PERFORMANCE: Recalculating every 100ms
    workingProxies := 0
    for _, result := range s.results {
        if result.Working {
            workingProxies++
        }
    }

    // ⚠️ PERFORMANCE: Another loop every 100ms
    var totalSpeed time.Duration
    var speedCount int
    for _, result := range s.results {
        if result.Speed > 0 {
            totalSpeed += result.Speed
            speedCount++
        }
    }
```

**Problem:**
- Iterating through all results every 100ms
- With 1000 proxies, that's 10,000 iterations per second
- O(n) computation on every tick
- Locks mutex for entire duration

**Impact:** Moderate - CPU usage, slower UI on large lists

**Solution:**
```go
// Update metrics incrementally, not by recalculating
case proxyCheckedMsg:
    s.results = append(s.results, msg.result)
    s.view.Current++

    // Incremental update - O(1) instead of O(n)
    if msg.result.Working {
        s.view.Working++
    } else {
        s.view.Failed++
    }

    // Running average for speed - O(1)
    if msg.result.Speed > 0 {
        s.speedSum += msg.result.Speed
        s.speedCount++
        s.view.AvgSpeed = s.speedSum / time.Duration(s.speedCount)
    }
```

---

### 6. 🟡 MODERATE: Mutex in Model

**Location:** [cmd/proxyhawk/main.go:38-41](cmd/proxyhawk/main.go#L38-L41)

```go
type AppState struct {
    mutex    sync.Mutex  // ⚠️ Sign of architectural issue
    // ...
}
```

**Problem:**
- Bubble Tea models should never need mutexes
- Indicates shared state between goroutines and Update()
- Makes testing difficult
- Sign that Elm architecture is being violated

**Impact:** Moderate - Architectural debt, hard to maintain

**Solution:**
Remove mutex entirely by following Elm architecture:
1. All state updates happen in Update()
2. Worker goroutines send messages, don't modify state
3. Single-threaded state updates = no races = no mutex needed

---

### 7. 🟡 MODERATE: No Component Isolation

**Location:** [internal/ui/view.go:8-72](internal/ui/view.go#L8-L72)

```go
func (v *View) Render() string {
    // ⚠️ Components directly access parent state
    statsBar := &StatsBarComponent{
        Current:  v.Current,  // Passing individual fields
        Total:    v.Total,
        Working:  v.Working,
        // ...
    }
```

**Problem:**
- Components don't have their own Models
- Parent must manually pass all fields
- No way for components to handle their own messages
- Can't compose components easily

**Impact:** Moderate - Harder to test, less reusable

**Better Pattern:**
```go
// Each component is a full tea.Model
type StatsBarModel struct {
    current  int
    total    int
    working  int
    failed   int
    active   int
    avgSpeed time.Duration
    width    int
}

func (m StatsBarModel) Update(msg tea.Msg) (StatsBarModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
    case statsUpdateMsg:
        m.current = msg.current
        m.working = msg.working
        // ...
    }
    return m, nil
}

func (m StatsBarModel) View() string {
    // Render based on own state
}

// Parent delegates to child
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Update children
    var cmd tea.Cmd
    s.statsBar, cmd = s.statsBar.Update(msg)
    cmds = append(cmds, cmd)

    return s, tea.Batch(cmds...)
}
```

---

### 8. 🟢 MINOR: View Method Conditional Logic

**Location:** [cmd/proxyhawk/main.go:609-617](cmd/proxyhawk/main.go#L609-L617)

```go
func (s *AppState) View() string {
    if s.view.Mode == ui.ModeDebug {
        return s.view.RenderDebug()
    }
    if s.view.Mode == ui.ModeVerbose {
        return s.view.RenderVerbose()
    }
    return s.view.RenderDefault()
}
```

**Problem:**
- Wrapper around wrapper
- AppState.View() just delegates to view.Render*()
- Could be simplified

**Impact:** Low - Minor code smell

**Solution:**
```go
// Just call view.Render() directly
func (s *AppState) View() string {
    return s.view.Render()  // View handles mode internally
}

// internal/ui/view.go already has Render() method that checks Mode
```

---

### 9. 🟢 MINOR: Context Cancellation Not Integrated

**Location:** [cmd/proxyhawk/main.go:664-666](cmd/proxyhawk/main.go#L664-L666)

```go
select {
case <-s.ctx.Done():
    // Worker exits but tea.Program doesn't know
    return
}
```

**Problem:**
- Workers respect context cancellation
- But tea.Program quits separately (via tea.Quit)
- No coordination between the two
- Could have orphaned goroutines

**Impact:** Low - Potential resource leak on quit

**Solution:**
```go
case tea.KeyMsg:
    if msg.String() == "q" {
        // Cancel context first
        s.cancel()
        // Then quit Bubble Tea
        return s, tea.Quit
    }

// Or better: workers send completion messages
case allWorkersCompleteMsg:
    return s, tea.Quit
```

---

### 10. 🟢 MINOR: No Tests

**Location:** Everywhere

**Problem:**
- No tests for Update() function
- No tests for View() rendering
- No tests for message handling
- All pure functions - should be very testable!

**Impact:** Low (currently) - Technical debt

**Example Test:**
```go
func TestQuitOnQ(t *testing.T) {
    model := initialModel()
    msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}

    _, cmd := model.Update(msg)

    // Should return tea.Quit command
    if cmd == nil {
        t.Fatal("expected quit command")
    }
}

func TestProxyCountUpdate(t *testing.T) {
    model := initialModel()
    model.view.Total = 100

    // Simulate proxy check completion
    msg := proxyCheckedMsg{
        result: &proxy.ProxyResult{Working: true},
    }

    newModel, _ := model.Update(msg)
    updated := newModel.(AppState)

    if updated.view.Current != 1 {
        t.Errorf("expected current=1, got %d", updated.view.Current)
    }
    if updated.view.Working != 1 {
        t.Errorf("expected working=1, got %d", updated.view.Working)
    }
}
```

---

## Positive Aspects ✅

### Things Done Right

1. **Component Architecture** - Good separation with Component interface
2. **Lip Gloss Styling** - Proper use of styles, adaptive colors
3. **Progress Component** - Using bubbles/progress correctly
4. **Clear View Hierarchy** - Header, stats, progress, checks, footer
5. **Debug Mode Support** - Nice tiered verbosity levels
6. **Spinner Animation** - Simple but effective active indicator
7. **Style Centralization** - All styles in styles.go
8. **Responsive Stats** - Good conditional display (hide if zero)
9. **Active Check Limiting** - MaxVisible prevents overflow
10. **Clean Render Methods** - Components have focused render logic

---

## Recommended Refactoring Plan

### Phase 1: Critical Fixes (Must Do)

**Priority: Immediate - Fixes broken behavior**

#### 1.1 Remove Goroutine from Init()
```go
// Before
func (s *AppState) Init() tea.Cmd {
    go s.startChecking()  // ❌
    return tickerCmd
}

// After
func (s *AppState) Init() tea.Cmd {
    return tea.Batch(
        s.startCheckingCmd(),  // ✅ Returns tea.Cmd
        timer.Init(),          // ✅ Use bubbles/timer
    )
}
```

#### 1.2 Replace Manual Ticker with bubbles/timer
```bash
go get github.com/charmbracelet/bubbles/timer
```

```go
import "github.com/charmbracelet/bubbles/timer"

type AppState struct {
    ticker timer.Model
    // Remove: updateChan, mutex
}

func initialModel() AppState {
    return AppState{
        ticker: timer.NewWithInterval(100*time.Millisecond, 100*time.Millisecond),
    }
}
```

#### 1.3 Add WindowSizeMsg Handler
```go
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        s.width = msg.Width
        s.height = msg.Height
        s.view.Progress.Width = msg.Width - 4
        return s, nil
```

#### 1.4 Convert Worker Communication to Messages
```go
// Define message types
type proxyCheckStartedMsg struct{ proxy string }
type proxyCheckedMsg struct {
    proxy  string
    result *proxy.ProxyResult
}
type allChecksCompleteMsg struct{}

// Workers send messages instead of using channels
func checkProxyCmd(proxy string, checker *proxy.Checker) tea.Cmd {
    return func() tea.Msg {
        result := checker.Check(proxy)
        return proxyCheckedMsg{proxy: proxy, result: result}
    }
}

// Update handles all state changes
case proxyCheckedMsg:
    s.results = append(s.results, msg.result)
    s.view.Current++
    if msg.result.Working {
        s.view.Working++
    } else {
        s.view.Failed++
    }
```

**Estimated Effort:** 4-6 hours
**Impact:** Fixes UI freezing, removes race conditions, enables testing

---

### Phase 2: Performance Improvements (Should Do)

**Priority: High - Improves performance significantly**

#### 2.1 Incremental Metrics
```go
// Instead of recalculating every tick:
// Before
for _, result := range s.results {  // O(n) every 100ms
    if result.Working { workingProxies++ }
}

// After
case proxyCheckedMsg:  // O(1) per check
    if msg.result.Working {
        s.view.Working++
    }
```

#### 2.2 Remove Mutex
Once worker communication uses messages, no shared state = no mutex needed.

```go
type AppState struct {
    // Remove: mutex sync.Mutex
}

// All Updates happen in Update() - single-threaded, no races
```

**Estimated Effort:** 2-3 hours
**Impact:** Lower CPU usage, cleaner architecture

---

### Phase 3: Component Modernization (Nice to Have)

**Priority: Medium - Improves maintainability**

#### 3.1 Make Components Full Models
```go
// Each component becomes a tea.Model
type StatsBarModel struct {
    // State
    current, total, working, failed, active int
    avgSpeed time.Duration
}

func (m StatsBarModel) Update(msg tea.Msg) (StatsBarModel, tea.Cmd) {
    switch msg := msg.(type) {
    case statsUpdateMsg:
        m.current = msg.current
        // ...
    }
    return m, nil
}

func (m StatsBarModel) View() string {
    // Render from own state
}
```

#### 3.2 Proper Message Delegation
```go
func (s *AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Update all child components
    var cmd tea.Cmd
    s.statsBar, cmd = s.statsBar.Update(msg)
    cmds = append(cmds, cmd)

    s.progressBar, cmd = s.progressBar.Update(msg)
    cmds = append(cmds, cmd)

    // ... handle parent-specific messages

    return s, tea.Batch(cmds...)
}
```

**Estimated Effort:** 4-6 hours
**Impact:** Better testing, easier composition, more maintainable

---

### Phase 4: Testing (Recommended)

**Priority: Medium - Long-term benefit**

#### 4.1 Unit Tests for Update()
```go
func TestProxyCheckHandling(t *testing.T) {
    tests := []struct{
        name string
        msg tea.Msg
        want AppState
    }{
        // Test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            model := initialModel()
            got, _ := model.Update(tt.msg)
            // assertions
        })
    }
}
```

#### 4.2 Snapshot Tests for View()
```go
func TestViewRendering(t *testing.T) {
    model := initialModel()
    model.view.Current = 50
    model.view.Total = 100
    model.view.Working = 30

    output := model.View()

    // Check output contains expected elements
    if !strings.Contains(output, "50/100") {
        t.Error("missing progress count")
    }
}
```

**Estimated Effort:** 3-4 hours
**Impact:** Confidence in refactoring, catches regressions

---

## Implementation Priority

### Immediate (Next Session)
1. ✅ Fix ticker (already done)
2. Add WindowSizeMsg handler
3. Remove goroutine from Init()

### Next Sprint
4. Convert to message-based communication
5. Remove mutex
6. Incremental metrics

### Future Iteration
7. Component models
8. Add tests
9. Polish and optimization

---

## Code Quality Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Elm Architecture Compliance | 60% | 95% | 🔴 Needs work |
| Component Isolation | 70% | 90% | 🟡 Good start |
| Test Coverage | 0% | 70% | 🔴 Missing |
| Performance (1000 proxies) | Good | Excellent | 🟡 Can improve |
| WindowResize Support | 0% | 100% | 🔴 Missing |
| Mutex Usage | Yes | No | 🔴 Should remove |

---

## Comparison to Bubble Tea Best Practices

| Practice | ProxyHawk | Status | Priority |
|----------|-----------|--------|----------|
| Pure Update() function | ❌ Has I/O via goroutines | Fix | High |
| No side effects in View() | ✅ View is pure | Good | - |
| Handle WindowSizeMsg | ❌ Missing | Fix | High |
| Provide quit path | ✅ q and Ctrl+C work | Good | - |
| Use tea.Cmd for side effects | ❌ Manual goroutines | Fix | High |
| tea.Batch for multiple commands | ✅ Used correctly | Good | - |
| Component composition | 🟡 Partial - not full models | Improve | Medium |
| Adaptive colors | ✅ Using Lip Gloss properly | Good | - |
| Responsive layout | ❌ No resize handling | Fix | High |
| Bubbles components | ✅ Using progress, timer planned | Good | - |

---

## Summary & Next Steps

### What's Working Well
- Clean visual design
- Good component separation
- Proper Lip Gloss usage
- User-friendly interface

### What Needs Fixing
- **Critical:** Goroutine management (violates Elm arch)
- **Critical:** WindowSizeMsg (breaks on resize)
- **Important:** Message-based communication
- **Important:** Remove mutex

### Recommended Action Plan

**Week 1: Critical Fixes**
1. Add WindowSizeMsg handling (30 min)
2. Convert Init() to proper tea.Cmd (2 hours)
3. Implement message-based worker communication (3-4 hours)

**Week 2: Architecture Cleanup**
4. Remove mutex by eliminating shared state (2 hours)
5. Implement incremental metrics (2 hours)
6. Add basic tests (3 hours)

**Week 3: Polish**
7. Convert components to full models (4 hours)
8. Add comprehensive tests (4 hours)
9. Performance profiling and optimization (2 hours)

**Total Effort:** ~22-24 hours for complete refactoring

### Immediate Quick Win

The fastest improvement with biggest impact:

```go
// Add this to Update() right after the switch statement
case tea.WindowSizeMsg:
    s.width = msg.Width
    s.height = msg.Height
    s.view.Progress.Width = msg.Width - 4
    return s, nil
```

**Time:** 5 minutes
**Impact:** No more broken layouts on resize

---

## Conclusion

The ProxyHawk TUI is **functionally correct and visually appealing**, but has architectural debt that will cause issues long-term. The main problems stem from trying to integrate traditional concurrent Go patterns (goroutines, mutexes, channels) with the Elm architecture that Bubble Tea enforces.

The good news: The issues are well-understood and fixable. Following the phased refactoring plan above will result in a TUI that is:
- ✅ More performant
- ✅ Easier to test
- ✅ More maintainable
- ✅ Fully compliant with Bubble Tea best practices
- ✅ More composable and extensible

**Grade After Refactoring: A**

The foundation is solid. With the recommended fixes, ProxyHawk will have a best-in-class TUI implementation.
