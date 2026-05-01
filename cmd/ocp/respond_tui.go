package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/csellis/ocp/internal/storage"
)

// respondMode is the current input target inside the model. Menu mode
// reads single-key shortcuts; the input modes feed keystrokes to a
// textinput so sub-prompts get real line editing and a real ESC.
type respondMode int

const (
	modeMenu respondMode = iota
	modeReasonInput
	modeSynonymInput
)

// respondModel walks one observation. The wrapper in makeTUIPrompt
// builds a fresh respondModel per call so the queue is just "loop and
// run a tea.Program."
type respondModel struct {
	state       storage.IssueState
	mode        respondMode
	input       textinput.Model
	showHelp    bool
	showDetails bool
	flash       string
	action      replyAction
	quitting    bool
	styles      homeStyles
}

func newRespondModel(state storage.IssueState, color bool) respondModel {
	return respondModel{
		state:  state,
		styles: newHomeStyles(color),
	}
}

func (m respondModel) Init() tea.Cmd { return nil }

func (m respondModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if m.mode != modeMenu {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	switch m.mode {
	case modeMenu:
		return m.updateMenu(keyMsg)
	default:
		return m.updateInput(msg, keyMsg)
	}
}

func (m respondModel) updateMenu(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.flash = ""
	switch keyMsg.String() {
	case "ctrl+c", "esc", "q":
		m.quitting = true
		return m, tea.Quit
	case "?", "h":
		m.showHelp = !m.showHelp
		return m, nil
	case "d":
		m.showDetails = !m.showDetails
		return m, nil
	case "n", "enter":
		m.action = replyAction{kind: replyNone}
		return m, tea.Quit
	case "b":
		m.action = replyAction{kind: replyStandBy}
		return m, tea.Quit
	case "c":
		m.mode = modeReasonInput
		m.input = newSubInput("reason")
		return m, textinput.Blink
	case "s":
		m.mode = modeSynonymInput
		m.input = newSubInput("term")
		return m, textinput.Blink
	}
	m.flash = fmt.Sprintf("unknown choice %q; try c, s, b, d, ?, n, q", keyMsg.String())
	return m, nil
}

func (m respondModel) updateInput(msg tea.Msg, keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.cancelInput()
		return m, nil
	case tea.KeyEnter:
		v := strings.TrimSpace(m.input.Value())
		if v == "" {
			m.cancelInput()
			return m, nil
		}
		switch m.mode {
		case modeReasonInput:
			m.action = replyAction{kind: replyClose, value: v}
		case modeSynonymInput:
			m.action = replyAction{kind: replySynonym, value: v}
		}
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *respondModel) cancelInput() {
	m.mode = modeMenu
	m.input.Reset()
	m.flash = "(cancelled)"
}

func (m respondModel) View() string {
	var b strings.Builder
	b.WriteString(respondHeader(m.state, m.styles))
	b.WriteString("\n")
	b.WriteString(m.styles.dim.Render("[c]lose  [s]ynonym  [b] stand by  [d]etails  [?]help  [n]ext  [q]uit"))
	b.WriteString("\n")

	if m.showDetails {
		b.WriteString("\n")
		b.WriteString(detailsView(m.state, m.styles))
	}
	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(legendView(m.styles))
	}

	switch m.mode {
	case modeReasonInput:
		fmt.Fprintf(&b, "\n  reason (empty to cancel, esc to abort): %s\n", m.input.View())
	case modeSynonymInput:
		fmt.Fprintf(&b, "\n  term (empty to cancel, esc to abort): %s\n", m.input.View())
	}

	if m.flash != "" {
		fmt.Fprintf(&b, "  %s\n", m.styles.dim.Render(m.flash))
	}
	return b.String()
}

func newSubInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.Prompt = ""
	return ti
}

// makeTUIPrompt returns a prompter that walks one observation per call
// in a bubbletea program. Each call spins up a fresh tea.Program for
// the observation; the wrapper extracts the chosen action (or errQuit)
// from the final model. The legend prints once before the first program
// runs so first-time users learn the menu.
//
// Tests should drive the model directly (newRespondModel + Update);
// this wrapper is integration glue for the live terminal.
func makeTUIPrompt(stdin io.Reader, out io.Writer, color bool) prompter {
	styles := newHomeStyles(color)
	first := true
	return func(state storage.IssueState) (replyAction, error) {
		if first {
			fmt.Fprintln(out, legendView(styles))
			first = false
		}
		m := newRespondModel(state, color)
		finalM, err := tea.NewProgram(m,
			tea.WithInput(stdin),
			tea.WithOutput(out),
		).Run()
		if err != nil {
			return replyAction{}, fmt.Errorf("respond tui: %w", err)
		}
		rm, ok := finalM.(respondModel)
		if !ok {
			return replyAction{}, fmt.Errorf("respond tui: unexpected model type %T", finalM)
		}
		return extractAction(rm)
	}
}

// extractAction reads the result a respondModel ended with. The
// quitting flag is set only when the user pressed q/esc/ctrl+c at the
// menu (no action recorded); surface that as errQuit so runRespond
// breaks the queue loop. All other exits (next, stand-by, close,
// synonym) carry an action.
func extractAction(m respondModel) (replyAction, error) {
	if m.quitting {
		return replyAction{}, errQuit
	}
	return m.action, nil
}

// legendView explains each menu choice in plain words. Printed once
// per session by the wrapper, and again on demand inside the model
// when the user presses ?.
func legendView(styles homeStyles) string {
	var b strings.Builder
	fmt.Fprintln(&b, styles.bold.Render("Actions:"))
	fmt.Fprintln(&b, "  [c]lose      this isn't drift; archive with a reason")
	fmt.Fprintln(&b, "  [s]ynonym    add the term as a glossary synonym; archive")
	fmt.Fprintln(&b, "  [b] stand by leave open; revisit later")
	fmt.Fprintln(&b, "  [d]etails    show the full citation list")
	fmt.Fprintln(&b, "  [n]ext       skip without changing anything")
	fmt.Fprintln(&b, "  [q]uit       stop reviewing")
	return b.String()
}

// respondHeader is the at-a-glance one-liner shown above the menu.
// Reads structured fields from frontmatter; no body parsing.
func respondHeader(s storage.IssueState, styles homeStyles) string {
	term := s.Term
	if term == "" {
		term = "(no term recorded)"
	}
	canonical := s.Canonical
	if canonical == "" {
		canonical = "(no canonical recorded)"
	}
	density := ""
	if s.Files > 0 {
		density = fmt.Sprintf("  %s",
			styles.dim.Render(fmt.Sprintf("%d %s / %d %s",
				s.Files, pluralize("file", s.Files),
				s.Occurrences, pluralize("occurrence", s.Occurrences))))
	}
	reviewed := ""
	if !s.LastReviewed.IsZero() {
		reviewed = "  " + styles.dim.Render("reviewed "+s.LastReviewed.UTC().Format("2006-01-02"))
	}
	return fmt.Sprintf("%s   %s %s %s%s%s",
		styles.dim.Render(s.Ref.Path),
		styles.title.Render(term),
		styles.dim.Render("->"),
		styles.bold.Render(canonical),
		density,
		reviewed,
	)
}

func detailsView(s storage.IssueState, styles homeStyles) string {
	var b strings.Builder
	body := strings.TrimRight(s.Body, "\n")
	for _, line := range strings.Split(body, "\n") {
		fmt.Fprintf(&b, "    %s\n", styles.dim.Render(line))
	}
	return b.String()
}
