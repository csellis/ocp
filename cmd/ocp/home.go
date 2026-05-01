package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/csellis/ocp/internal/storage"
)

// homeChoice is the action a user picked from the home menu, or
// choiceQuit if they walked away. choiceNone means the program ended
// without a real selection (treat as quit).
type homeChoice int

const (
	choiceNone homeChoice = iota
	choiceScan
	choiceDrift
	choiceRespond
	choiceQuit
)

// homeAction is one row in the home menu. Stable ordering: scan, drift,
// respond. New actions append; existing keys do not change.
type homeAction struct {
	key    string
	label  string
	short  string
	long   string
	choice homeChoice
}

var homeActions = []homeAction{
	{"s", "scan", "seed or reload the glossary",
		"read or seed .ocp/glossary.md (start here on a fresh repo)", choiceScan},
	{"d", "drift", "look for drift in the working tree",
		"walk the working tree, file an observation per drift event", choiceDrift},
	{"r", "respond", "walk open observations",
		"walk open observations and decide on each (interactive)", choiceRespond},
}

// homeModel is the bubbletea Model for the home menu. One Model =
// one trip to the menu; runHome constructs a fresh Model on every loop.
type homeModel struct {
	state    homeState
	cursor   int
	showHelp bool
	choice   homeChoice
	styles   homeStyles
}

func newHomeModel(state homeState, color bool) homeModel {
	return homeModel{
		state:  state,
		styles: newHomeStyles(color),
	}
}

func (m homeModel) Init() tea.Cmd { return nil }

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c", "esc", "q":
		m.choice = choiceQuit
		return m, tea.Quit
	case "?", "h":
		m.showHelp = !m.showHelp
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(homeActions)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		m.choice = homeActions[m.cursor].choice
		return m, tea.Quit
	}
	for _, a := range homeActions {
		if keyMsg.String() == a.key {
			m.choice = a.choice
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m homeModel) View() string {
	var b strings.Builder
	fmt.Fprintln(&b, m.styles.title.Render("ocp"))
	fmt.Fprintln(&b)

	if !m.state.HasGlossary {
		b.WriteString(welcomeView(m.styles))
	} else {
		b.WriteString(statusView(m.state, m.styles))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, m.styles.bold.Render("What now?"))
	for i, a := range homeActions {
		line := fmt.Sprintf("[%s] %-7s %s", a.key, a.label, a.short)
		if i == m.cursor {
			fmt.Fprintf(&b, "%s%s\n",
				m.styles.selected.Render("> "),
				m.styles.selected.Render(line))
		} else {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	fmt.Fprintf(&b, "  %s\n",
		m.styles.dim.Render("[?] help   [q] quit   up/down or letter, enter to select"))
	if m.showHelp {
		fmt.Fprintln(&b)
		b.WriteString(helpView(m.styles))
	}
	return b.String()
}

// runHome is the interactive top-level menu. Each iteration:
//  1. snapshot project state
//  2. render the bubbletea menu, await a choice
//  3. dispatch the choice against `out`
//  4. divider, loop
//
// Returns when the user picks quit (or sends EOF / Ctrl-C / ESC).
func runHome(ctx context.Context, root string, out io.Writer, stdin *os.File) error {
	color := isTerminal(stdin) && isTerminal(os.Stdout)
	for {
		now := time.Now().UTC()
		state := readHomeState(ctx, root, now)
		m := newHomeModel(state, color)
		finalM, err := tea.NewProgram(m,
			tea.WithInput(stdin),
			tea.WithOutput(out),
		).Run()
		if err != nil {
			return fmt.Errorf("home: %w", err)
		}
		hm, ok := finalM.(homeModel)
		if !ok {
			return fmt.Errorf("home: unexpected model type %T", finalM)
		}
		if hm.choice == choiceQuit || hm.choice == choiceNone {
			return nil
		}
		if err := dispatchHome(ctx, root, out, stdin, hm.choice, now, color); err != nil {
			return err
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, m.styles.dim.Render("---"))
		fmt.Fprintln(out)
	}
}

// dispatchHome runs the subcommand the user picked. Pulled out of
// runHome so tests can drive each branch without spinning up a tea.Program.
func dispatchHome(ctx context.Context, root string, out io.Writer, stdin io.Reader, choice homeChoice, now time.Time, color bool) error {
	switch choice {
	case choiceScan:
		return runScan(ctx, out, root, now)
	case choiceDrift:
		return runDrift(ctx, out, root, now)
	case choiceRespond:
		return runRespond(ctx, out, root, now, makeTUIPrompt(stdin, out, color))
	}
	return nil
}

// homeState is the at-a-glance summary the home menu shows. Pulled
// from storage on demand; cheap (a glossary read, a directory list,
// a log stat).
type homeState struct {
	HasGlossary    bool
	GlossaryTerms  int
	OpenObs        int
	OldestReviewed time.Time
	LogEntries     int
}

func readHomeState(ctx context.Context, root string, _ time.Time) homeState {
	fs := storage.New(root)
	var s homeState

	if g, err := fs.LoadGlossary(ctx, storage.RepoID("")); err == nil {
		s.HasGlossary = true
		s.GlossaryTerms = len(g.Terms)
	}

	if refs, err := fs.LoadOpenIssues(ctx, storage.RepoID("")); err == nil {
		s.OpenObs = len(refs)
		for _, ref := range refs {
			st, err := fs.LoadIssue(ctx, storage.RepoID(""), ref)
			if err != nil {
				continue
			}
			if st.LastReviewed.IsZero() {
				continue
			}
			if s.OldestReviewed.IsZero() || st.LastReviewed.Before(s.OldestReviewed) {
				s.OldestReviewed = st.LastReviewed
			}
		}
	}

	if data, err := os.ReadFile(root + "/.ocp/log.md"); err == nil {
		s.LogEntries = strings.Count(string(data), "\n## ")
		if strings.HasPrefix(string(data), "## ") {
			s.LogEntries++
		}
	}
	return s
}

func statusView(s homeState, styles homeStyles) string {
	var b strings.Builder
	fmt.Fprintln(&b, styles.bold.Render("State:"))
	fmt.Fprintf(&b, "  glossary    %d terms\n", s.GlossaryTerms)
	fmt.Fprintf(&b, "  open obs    %d", s.OpenObs)
	if s.OpenObs > 0 && !s.OldestReviewed.IsZero() {
		fmt.Fprintf(&b, "  %s",
			styles.dim.Render(fmt.Sprintf("(oldest reviewed %s)", humanizeAge(s.OldestReviewed))))
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "  log         %d %s\n", s.LogEntries, pluralize("entry", s.LogEntries))
	return b.String()
}

func helpView(styles homeStyles) string {
	var b strings.Builder
	fmt.Fprintln(&b, styles.bold.Render("Actions:"))
	fmt.Fprintln(&b, "  [s] scan        read or seed .ocp/glossary.md (start here on a fresh repo)")
	fmt.Fprintln(&b, "  [d] drift       walk the working tree, file an observation per drift event")
	fmt.Fprintln(&b, "  [r] respond     walk open observations and decide on each (interactive)")
	fmt.Fprintln(&b, "  [?] help        show this list")
	fmt.Fprintln(&b, "  [q] quit        leave OCP")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, styles.dim.Render("Background: README.md, docs/THESIS.md, AGENTS.md."))
	return b.String()
}

// welcomeView runs on the first home open in a repo (no .ocp/glossary.md
// yet). Sets context for someone who has just installed OCP and run it
// for the first time. Once scan creates the glossary, this never shows
// again.
func welcomeView(styles homeStyles) string {
	var b strings.Builder
	fmt.Fprintln(&b, "  "+styles.bold.Render("Welcome."))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "  OCP watches a codebase for drift in its ubiquitous language: the")
	fmt.Fprintln(&b, "  canonical names a team has agreed on for the concepts in their domain.")
	fmt.Fprintln(&b, "  It does not write code. It surfaces single observations and updates")
	fmt.Fprintln(&b, "  its own .ocp/ state. Speak rarely; speak deliberately.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "  "+styles.bold.Render("Three actions:"))
	fmt.Fprintln(&b, "    "+styles.cyan.Render("scan")+"     read or seed .ocp/glossary.md (start here)")
	fmt.Fprintln(&b, "    "+styles.cyan.Render("drift")+"    walk the working tree, file an observation per drift event")
	fmt.Fprintln(&b, "    "+styles.cyan.Render("respond")+"  walk open observations and decide on each")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "  "+styles.dim.Render("Background: README.md, docs/THESIS.md, AGENTS.md."))
	return b.String()
}

// humanizeAge returns a coarse "N days ago" for the home status. Times
// are rounded to whole days because finer precision is noise here.
func humanizeAge(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%d days ago", int(d/(24*time.Hour)))
	}
}

// homeStyles bundles the lipgloss styles the home view uses. Off-mode
// (color=false) returns plain styles so test assertions stay ANSI-free.
type homeStyles struct {
	title    lipgloss.Style
	bold     lipgloss.Style
	dim      lipgloss.Style
	cyan     lipgloss.Style
	selected lipgloss.Style
}

func newHomeStyles(color bool) homeStyles {
	if !color {
		plain := lipgloss.NewStyle()
		return homeStyles{
			title: plain, bold: plain, dim: plain,
			cyan: plain, selected: plain,
		}
	}
	return homeStyles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
		bold:     lipgloss.NewStyle().Bold(true),
		dim:      lipgloss.NewStyle().Faint(true),
		cyan:     lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")),
	}
}
