package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jayyao97/snap-commit/internal/storage"
)

// SnapshotListMode controls how the interactive UI behaves.
type SnapshotListMode int

// DeleteHandler handles deleting a snapshot from persistent storage.
type DeleteHandler func(*storage.Snapshot) error

const (
	// SnapshotListBrowse shows snapshots without returning a selection.
	SnapshotListBrowse SnapshotListMode = iota
	// SnapshotListSelect allows selecting a snapshot (used by restore).
	SnapshotListSelect
)

// ErrAborted indicates that the user left the interactive UI without selecting.
var ErrAborted = errors.New("operation cancelled")

// BrowseSnapshots launches an interactive view for the provided snapshots.
func BrowseSnapshots(
	snapshots []storage.Snapshot,
	title string,
	showRepo bool,
	deleteHandler DeleteHandler,
) error {
	_, err := runSnapshotList(snapshots, title, showRepo, SnapshotListBrowse, deleteHandler)
	if errors.Is(err, ErrAborted) {
		return nil
	}
	return err
}

// SelectSnapshot launches an interactive selector and returns the chosen snapshot.
func SelectSnapshot(snapshots []storage.Snapshot, title string, showRepo bool) (*storage.Snapshot, error) {
	return runSnapshotList(snapshots, title, showRepo, SnapshotListSelect, nil)
}

func runSnapshotList(
	snapshots []storage.Snapshot,
	title string,
	showRepo bool,
	mode SnapshotListMode,
	deleteHandler DeleteHandler,
) (*storage.Snapshot, error) {
	snapshotRefs := make([]*storage.Snapshot, len(snapshots))
	items := make([]list.Item, len(snapshots))
	for i := range snapshots {
		snapshotRefs[i] = &snapshots[i]
		items[i] = snapshotItem{
			index:    i + 1,
			showRepo: showRepo,
			snapshot: snapshotRefs[i],
		}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	model := newSnapshotListModel(items, snapshotRefs, delegate, title, mode, showRepo, deleteHandler)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(snapshotListModel)
	if mode == SnapshotListSelect {
		if result.selection == nil {
			return nil, ErrAborted
		}
		return result.selection, nil
	}

	return nil, ErrAborted
}

type snapshotItem struct {
	index    int
	showRepo bool
	snapshot *storage.Snapshot
}

func (s snapshotItem) Title() string {
	return fmt.Sprintf("[%d] %s", s.index, s.snapshot.Message)
}

func (s snapshotItem) Description() string {
	timestamp := s.snapshot.Timestamp.Format("2006-01-02 15:04:05")
	desc := fmt.Sprintf("%s • %s", truncateSHA(s.snapshot.CommitSHA), timestamp)
	if s.showRepo {
		desc = fmt.Sprintf("%s • %s", desc, s.snapshot.RepoRoot)
	}
	return desc
}

func (s snapshotItem) FilterValue() string {
	return strings.ToLower(fmt.Sprintf(
		"%s %s %s %s",
		s.snapshot.Message,
		s.snapshot.ID,
		s.snapshot.CommitSHA,
		s.snapshot.RepoRoot,
	))
}

type snapshotListModel struct {
	list          list.Model
	mode          SnapshotListMode
	selection     *storage.Snapshot
	cancelled     bool
	showRepo      bool
	snapshots     []*storage.Snapshot
	deleteHandler DeleteHandler
	statusMessage string
}

func newSnapshotListModel(
	items []list.Item,
	snapshots []*storage.Snapshot,
	delegate list.DefaultDelegate,
	title string,
	mode SnapshotListMode,
	showRepo bool,
	deleteHandler DeleteHandler,
) snapshotListModel {
	l := list.New(items, delegate, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetShowPagination(true)
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	return snapshotListModel{
		list:          l,
		mode:          mode,
		showRepo:      showRepo,
		snapshots:     snapshots,
		deleteHandler: deleteHandler,
	}
}

func (m snapshotListModel) Init() tea.Cmd {
	return nil
}

func (m snapshotListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h := msg.Height - 6
		if h < 5 {
			h = 5
		}
		m.list.SetSize(msg.Width, h)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if m.mode == SnapshotListSelect {
				if item, ok := m.list.SelectedItem().(snapshotItem); ok {
					m.selection = item.snapshot
					m.cancelled = false
					return m, tea.Quit
				}
			}
		case "d":
			if m.deleteHandler != nil {
				if item, ok := m.list.SelectedItem().(snapshotItem); ok && item.snapshot != nil {
					if err := m.deleteHandler(item.snapshot); err != nil {
						m.statusMessage = fmt.Sprintf("Delete failed: %v", err)
					} else {
						m.statusMessage = fmt.Sprintf("Deleted snapshot %s", item.snapshot.ID)
						m.removeSnapshot(item.snapshot)
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m snapshotListModel) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.detailView())
	b.WriteString("\n")
	b.WriteString(m.helpText())
	if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(m.statusMessage)
	}
	return b.String()
}

func (m snapshotListModel) detailView() string {
	item, ok := m.list.SelectedItem().(snapshotItem)
	if !ok || item.snapshot == nil {
		if len(m.snapshots) == 0 {
			return "No snapshots available"
		}
		return "No snapshot selected"
	}

	lines := []string{
		fmt.Sprintf("ID:        %s", item.snapshot.ID),
		fmt.Sprintf("Commit:    %s", item.snapshot.CommitSHA),
		fmt.Sprintf("Timestamp: %s", item.snapshot.Timestamp.Format(time.RFC3339)),
		fmt.Sprintf("Message:   %s", item.snapshot.Message),
	}
	if item.showRepo {
		lines = append(lines, fmt.Sprintf("Repository: %s", item.snapshot.RepoRoot))
	}
	return strings.Join(lines, "\n")
}

func (m snapshotListModel) helpText() string {
	deleteHint := ""
	if m.deleteHandler != nil {
		deleteHint = " • d to delete"
	}

	if m.mode == SnapshotListSelect {
		return "↑/↓ to navigate • Enter to restore • Esc to cancel" + deleteHint
	}
	return "↑/↓ to navigate • Esc/q to exit" + deleteHint
}

func (m *snapshotListModel) removeSnapshot(target *storage.Snapshot) {
	idx := -1
	for i, snap := range m.snapshots {
		if snap == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	m.snapshots = append(m.snapshots[:idx], m.snapshots[idx+1:]...)
	m.refreshItems()
}

func (m *snapshotListModel) refreshItems() {
	items := make([]list.Item, len(m.snapshots))
	for i, snap := range m.snapshots {
		items[i] = snapshotItem{
			index:    i + 1,
			showRepo: m.showRepo,
			snapshot: snap,
		}
	}

	m.list.SetItems(items)

	if len(items) == 0 {
		m.list.ResetSelected()
		return
	}

	if cursor := m.list.Index(); cursor >= len(items) {
		m.list.Select(len(items) - 1)
	}
}

func truncateSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
