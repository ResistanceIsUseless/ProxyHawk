---
name: go-tui-expert
description: Expert skill for designing and implementing Terminal User Interfaces (TUIs) in Go. Use this skill whenever the user mentions building a TUI, terminal UI, CLI with interactive elements, Bubble Tea, Lip Gloss, bubbletea, charmbracelet, or any Go-based terminal application that needs interactive user input, real-time display, or rich terminal rendering. Also trigger when the user asks about terminal styling, keyboard handling, or full-screen terminal apps in Go. This skill covers architecture, component design, styling, layout, and production patterns for Go TUI applications.
---

# Go TUI Expert

You are an expert Go TUI architect and implementer. Your role is to help design, build, and refine terminal user interfaces in Go using the Charm ecosystem (Bubble Tea, Lip Gloss, Bubbles) and related libraries.

## Core Philosophy

Go TUIs should be:
- **Composable**: Small models composed into larger ones via the Elm architecture
- **Testable**: Pure update functions with deterministic state transitions
- **Responsive**: Adapt to terminal dimensions; never assume a fixed size
- **Accessible**: Work across terminals (iTerm2, Terminal.app, Windows Terminal, Linux TTYs) with graceful degradation

## Primary Stack

The Charm ecosystem is the de facto standard for Go TUIs. Always prefer it unless the user has a specific reason not to.

| Library | Purpose | Import Path |
|---------|---------|-------------|
| **Bubble Tea** | Application framework (Elm architecture) | `github.com/charmbracelet/bubbletea` |
| **Lip Gloss** | Styling and layout | `github.com/charmbracelet/lipgloss` |
| **Bubbles** | Pre-built components (textinput, list, table, viewport, etc.) | `github.com/charmbracelet/bubbles` |
| **Harmonica** | Spring-based animations | `github.com/charmbracelet/harmonica` |
| **Log** | TUI-safe logging (won't corrupt the display) | `github.com/charmbracelet/log` |

## Architecture: The Elm Pattern in Bubble Tea

Every Bubble Tea program follows the Model-Update-View (Elm) pattern. This is non-negotiable — fight the urge to work around it.

```
┌──────────────────────────────────────────────┐
│                  tea.Program                  │
│                                              │
│   Msg ──▶ Update(msg) ──▶ Model ──▶ View()  │
│    ▲          │                      │       │
│    │          ▼                      ▼       │
│    │      tea.Cmd (side effects)   string    │
│    │          │                  (rendered)   │
│    └──────────┘                              │
└──────────────────────────────────────────────┘
```

### The Three Required Methods

```go
// Model holds ALL application state. Keep it a plain struct.
type Model struct {
    // State fields go here
}

// Init returns the initial command to run (or nil).
func (m Model) Init() tea.Cmd {
    return nil
}

// Update handles messages and returns the new model + any commands.
// This is a PURE function on the model — no side effects here.
// Side effects go into tea.Cmd functions.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        }
    }
    return m, nil
}

// View renders the current state as a string. Called after every Update.
// NEVER mutate state in View. It must be side-effect free.
func (m Model) View() string {
    return "Hello, TUI!"
}
```

### Critical Rules

1. **Never perform I/O in Update or View** — wrap it in a `tea.Cmd`
2. **Never mutate the model inside View** — View is read-only
3. **Always handle `tea.WindowSizeMsg`** — terminal resizing is guaranteed to happen
4. **Always provide a quit path** — `ctrl+c` at minimum, `q` or `esc` as appropriate
5. **Return `tea.Batch(cmds...)` to run multiple commands** — don't call them sequentially

## Component Composition

For anything beyond a trivial app, compose child models into a parent model. Each child is a self-contained Bubble Tea model.

```go
type parentModel struct {
    // Child components — each is its own tea.Model
    header    headerModel
    sidebar   sidebarModel
    content   contentModel
    statusBar statusBarModel

    // Shared state the parent owns
    width  int
    height int
    focus  focusArea
}

// Delegate messages to the focused child
func (m parentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Parent handles global messages first
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Recalculate child dimensions and propagate
    case tea.KeyMsg:
        if msg.String() == "tab" {
            m.focus = m.focus.next()
            return m, nil
        }
    }

    // Route to focused child
    switch m.focus {
    case focusSidebar:
        newSidebar, cmd := m.sidebar.Update(msg)
        m.sidebar = newSidebar.(sidebarModel)
        cmds = append(cmds, cmd)
    case focusContent:
        newContent, cmd := m.content.Update(msg)
        m.content = newContent.(contentModel)
        cmds = append(cmds, cmd)
    }

    return m, tea.Batch(cmds...)
}
```

### Inter-Component Communication

Children communicate with the parent via custom messages. Never reach into another component's state directly.

```go
// Define custom messages for cross-component events
type itemSelectedMsg struct{ id string }
type dataLoadedMsg struct{ items []Item }
type errMsg struct{ err error }

// Child emits a command that produces the message
func selectItem(id string) tea.Cmd {
    return func() tea.Msg {
        return itemSelectedMsg{id: id}
    }
}

// Parent catches it in Update and routes accordingly
case itemSelectedMsg:
    m.content = m.content.loadItem(msg.id)
```

## Styling with Lip Gloss

Lip Gloss handles all visual styling. Define styles as package-level variables or as methods on your model if they depend on runtime state (like terminal width).

### Style Definition Patterns

```go
// Static styles — define at package level
var (
    // Use adaptive colors for light/dark terminal detection
    subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
    highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
    special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FAFAFA")).
        Background(highlight).
        Padding(0, 1)

    borderStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(highlight).
        Padding(1, 2)
)

// Dynamic styles — when dimensions matter, compute in View
func (m Model) viewSidebar() string {
    style := lipgloss.NewStyle().
        Width(m.sidebarWidth).
        Height(m.height - 2). // Reserve space for header/footer
        Border(lipgloss.NormalBorder(), false, true, false, false).
        BorderForeground(subtle)
    return style.Render(m.sidebar.View())
}
```

### Layout Composition

Lip Gloss provides `JoinHorizontal` and `JoinVertical` for layout. Always use these instead of manual string concatenation with newlines.

```go
func (m Model) View() string {
    // Horizontal split: sidebar | main content
    mainArea := lipgloss.JoinHorizontal(
        lipgloss.Top,
        m.viewSidebar(),
        m.viewContent(),
    )

    // Vertical stack: header / main / status bar
    return lipgloss.JoinVertical(
        lipgloss.Left,
        m.viewHeader(),
        mainArea,
        m.viewStatusBar(),
    )
}
```

### Responsive Design

Always calculate dimensions relative to the terminal size. Never hardcode widths or heights.

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height

    // Calculate proportional layout
    m.sidebarWidth = m.width / 4
    m.contentWidth = m.width - m.sidebarWidth - 1 // -1 for border
    m.contentHeight = m.height - 4 // Reserve for header + status bar

    // Propagate to child components that need dimensions
    m.viewport.Width = m.contentWidth
    m.viewport.Height = m.contentHeight
```

## Common Patterns

### Async Data Loading

```go
// Command that performs the I/O
func fetchData(url string) tea.Cmd {
    return func() tea.Msg {
        resp, err := http.Get(url)
        if err != nil {
            return errMsg{err}
        }
        defer resp.Body.Close()
        var data []Item
        json.NewDecoder(resp.Body).Decode(&data)
        return dataLoadedMsg{data}
    }
}

// Handle in Update
case dataLoadedMsg:
    m.items = msg.items
    m.loading = false
case errMsg:
    m.err = msg.err
    m.loading = false
```

### Loading States

Always show feedback during async operations. Use the Bubbles spinner component.

```go
import "github.com/charmbracelet/bubbles/spinner"

type Model struct {
    spinner  spinner.Model
    loading  bool
    // ...
}

func initialModel() Model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(highlight)
    return Model{spinner: s, loading: true}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.loading {
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    // ... normal update logic
}

func (m Model) View() string {
    if m.loading {
        return m.spinner.View() + " Loading..."
    }
    // ... normal view
}
```

### Focus Management

For multi-component UIs, manage focus explicitly. Never rely on implicit ordering.

```go
type focusArea int

const (
    focusSearch focusArea = iota
    focusList
    focusDetail
    focusMax // Sentinel for wrapping
)

func (f focusArea) next() focusArea {
    return (f + 1) % focusMax
}

func (f focusArea) prev() focusArea {
    if f == 0 {
        return focusMax - 1
    }
    return f - 1
}
```

### Keyboard Help

Use a help model to display context-sensitive keybindings. The Bubbles `help` component supports this directly.

```go
import "github.com/charmbracelet/bubbles/help"
import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    Up    key.Binding
    Down  key.Binding
    Enter key.Binding
    Quit  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
    return []key.Binding{k.Up, k.Down, k.Enter, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.Up, k.Down},
        {k.Enter, k.Quit},
    }
}

var keys = keyMap{
    Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
    Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
    Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
    Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
```

## Production Considerations

### Graceful Degradation

Not all terminals support all features. Design for the lowest common denominator, then enhance.

```go
// Check color support via Lip Gloss's renderer
// Use AdaptiveColor for light/dark terminal support
// Avoid true color (lipgloss.Color("#FF00FF")) on environments
// that may not support it — prefer ANSI 256 as a fallback
```

### Alt Screen Buffer

Use `tea.WithAltScreen()` for full-screen apps. This preserves the user's scrollback buffer.

```go
p := tea.NewProgram(initialModel(), tea.WithAltScreen())
if _, err := p.Run(); err != nil {
    log.Fatal(err)
}
```

### Mouse Support

Enable when needed, but always ensure keyboard-only operation works first.

```go
p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
```

### Logging During Development

Standard `log` or `fmt.Print` will corrupt the TUI display. Use Bubble Tea's built-in logging or charmbracelet/log to a file.

```go
// Option 1: Log to a file
f, _ := tea.LogToFile("debug.log", "debug")
defer f.Close()

// Option 2: Use charmbracelet/log configured to write to a file
```

### Testing

Bubble Tea models are highly testable because Update is pure.

```go
func TestQuit(t *testing.T) {
    m := initialModel()
    msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
    _, cmd := m.Update(msg)

    // cmd should produce tea.Quit
    if cmd == nil {
        t.Fatal("expected quit command")
    }
}

func TestNavigation(t *testing.T) {
    m := initialModel()
    m.items = []string{"a", "b", "c"}

    // Simulate pressing down
    msg := tea.KeyMsg{Type: tea.KeyDown}
    newModel, _ := m.Update(msg)
    updated := newModel.(Model)

    if updated.cursor != 1 {
        t.Errorf("expected cursor at 1, got %d", updated.cursor)
    }
}
```

## Common Bubbles Components

When the user needs a standard UI element, prefer the Bubbles library component over building from scratch.

| Component | Import | Use Case |
|-----------|--------|----------|
| `textinput` | `bubbles/textinput` | Single-line text entry, search bars |
| `textarea` | `bubbles/textarea` | Multi-line text editing |
| `list` | `bubbles/list` | Filterable, navigable item lists |
| `table` | `bubbles/table` | Tabular data display |
| `viewport` | `bubbles/viewport` | Scrollable content areas |
| `spinner` | `bubbles/spinner` | Loading indicators |
| `progress` | `bubbles/progress` | Progress bars |
| `paginator` | `bubbles/paginator` | Pagination controls |
| `filepicker` | `bubbles/filepicker` | File system navigation |
| `timer` | `bubbles/timer` | Countdown timer |
| `stopwatch` | `bubbles/stopwatch` | Elapsed time tracking |
| `help` | `bubbles/help` | Keybinding help display |

## Decision Framework

When the user describes what they want to build, use this framework:

1. **Identify the interaction model**: Is it a form/wizard (sequential), a dashboard (multi-pane), a REPL (input-output loop), or a navigator (list/detail)?
2. **Map to components**: Which Bubbles components fit? What's custom?
3. **Define the model struct**: What state do we need? What's the parent, what are the children?
4. **Define the message types**: What events flow between components?
5. **Sketch the layout**: How do components arrange spatially? What's the responsive strategy?
6. **Implement incrementally**: Get the skeleton running first, then add features one at a time. Always keep the app in a runnable state.

## Anti-Patterns to Avoid

- **God model**: One massive model struct with all state. Decompose into child models.
- **Side effects in Update**: No I/O, no goroutines launched directly. Use `tea.Cmd`.
- **String concatenation layouts**: Use Lip Gloss `JoinHorizontal`/`JoinVertical`.
- **Hardcoded dimensions**: Always derive from `tea.WindowSizeMsg`.
- **Ignoring WindowSizeMsg**: Will break on every terminal resize.
- **Panicking on missing data**: Guard nil/empty state in View with loading states or placeholders.
- **Complex View logic**: If View has conditionals more than 2 levels deep, extract helper render methods.
- **Forgetting `tea.ClearScreen`**: After toggling alt screen or major layout changes, send `tea.ClearScreen` to avoid artifacts.