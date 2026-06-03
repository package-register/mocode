package dialog

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/package-register/mocode/internal/ui/common"
	wechat "github.com/package-register/mocode/internal/wechat"
)

const (
	WeChatManagerID = "wechat_manager"
	// wechatManagerRefreshInterval is how often the dialog refreshes its
	// account list while open. Short enough to feel live, long enough to
	// avoid hammering the manager on every tick.
	wechatManagerRefreshInterval = 2 * time.Second
)

// WeChatManagerTickMsg is fired periodically while the WeChat Manager dialog
// is open. Receiving one triggers a refresh of the account list and the
// scheduling of the next tick.
type WeChatManagerTickMsg time.Time

// ActionWeChatReconnect asks the app to reconnect a WeChat account's poll loop.
type ActionWeChatReconnect struct {
	AccountID string
}

// ActionWeChatStart asks the app to (re)start a WeChat account's poll loop.
type ActionWeChatStart struct {
	AccountID string
}

// ActionWeChatStop asks the app to stop a WeChat account's poll loop.
type ActionWeChatStop struct {
	AccountID string
}

// ActionWeChatDelete asks the app to delete a WeChat account entirely.
type ActionWeChatDelete struct {
	AccountID string
}

// WeChatManager is a unified modal that lists all WeChat accounts and lets
// the user perform per-account actions (reconnect, start, stop, delete) as
// well as set the active account or open the QR login flow for a new one.
//
// While open, the dialog schedules a [WeChatManagerTickMsg] every
// [wechatManagerRefreshInterval] so the on-screen status stays in sync with
// background connection changes. The tick is self-scheduling: as long as
// the dialog receives ticks it will keep requesting the next one. When
// the dialog is closed (or another dialog is brought to front), the tick
// stops naturally because no one will call [WeChatManager.HandleMsg] again.
//
// Keyboard model:
//   - ↑/↓/j/k  : move selection
//   - enter    : set the highlighted account as the active one
//   - r        : reconnect (restart poll loop, or open QR if offline)
//   - s        : toggle start/stop poll loop
//   - d        : delete account (with confirmation prompt)
//   - n        : open QR login dialog to add a new account
//   - y/n      : confirm/cancel a pending delete
//   - esc/q    : close
type WeChatManager struct {
	com      *common.Common
	accounts []wechat.AccountInfo
	selected int

	// pendingDelete is the account ID awaiting delete confirmation (nil = none).
	pendingDelete string

	// tickEnabled records whether this dialog is currently scheduling ticks.
	// Used to avoid scheduling a second tick while one is already pending.
	tickEnabled bool

	keyMap struct {
		Up        key.Binding
		Down      key.Binding
		Select    key.Binding
		Reconnect key.Binding
		Toggle    key.Binding
		Delete    key.Binding
		New       key.Binding
		Close     key.Binding
		Confirm   key.Binding
		Cancel    key.Binding
	}
}

var _ Dialog = (*WeChatManager)(nil)

// NewWeChatManager creates a new WeChat Manager dialog. The returned
// [tea.Cmd] is the initial tick that should be scheduled by the caller
// (typically the UI that opened the dialog) to start the refresh loop.
func NewWeChatManager(com *common.Common) (*WeChatManager, tea.Cmd) {
	d := &WeChatManager{com: com, tickEnabled: true}
	d.keyMap.Up = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	d.keyMap.Down = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	d.keyMap.Select = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "set active"))
	d.keyMap.Reconnect = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reconnect"))
	d.keyMap.Toggle = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start/stop"))
	d.keyMap.Delete = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete"))
	d.keyMap.New = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new account"))
	d.keyMap.Close = CloseKey
	d.keyMap.Confirm = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm delete"))
	d.keyMap.Cancel = key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "cancel"))
	return d, d.scheduleTick()
}

// scheduleTick returns a tea.Cmd that, after the refresh interval, fires a
// [WeChatManagerTickMsg]. The dialog is responsible for rescheduling the
// next tick on every refresh; if the dialog is closed (or replaced) before
// the next tick fires, nothing reschedules it and the loop stops naturally.
func (d *WeChatManager) scheduleTick() tea.Cmd {
	return tea.Tick(wechatManagerRefreshInterval, func(t time.Time) tea.Msg {
		return WeChatManagerTickMsg(t)
	})
}

// stopTick marks the dialog as no longer scheduling ticks. Called when the
// dialog is closing so any in-flight tick cmd (already scheduled by the
// runtime) becomes a no-op when it eventually fires. Combined with the
// fact that the dialog is removed from the overlay, this guarantees the
// refresh loop terminates.
func (d *WeChatManager) stopTick() {
	d.tickEnabled = false
}

// ID implements Dialog.
func (d *WeChatManager) ID() string { return WeChatManagerID }

// refreshAccounts reloads the latest account list from the manager.
func (d *WeChatManager) refreshAccounts() {
	d.accounts = wechat.GetManager().List()
	// Clamp the selection in case accounts were added/removed.
	if d.selected >= len(d.accounts) {
		d.selected = max(0, len(d.accounts)-1)
	}
	if d.selected < 0 {
		d.selected = 0
	}
}

// HandleMsg implements Dialog.
func (d *WeChatManager) HandleMsg(msg tea.Msg) Action {
	// Periodic refresh: pull fresh state, then schedule the next tick.
	if _, ok := msg.(WeChatManagerTickMsg); ok {
		d.refreshAccounts()
		if !d.tickEnabled {
			return nil
		}
		return ActionCmd{Cmd: d.scheduleTick()}
	}

	d.refreshAccounts()
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// If we previously stopped the tick (e.g. when the user pressed [n]
		// to launch the QR login flow), resume it on the next user
		// interaction. This keeps the dialog "live" again without
		// requiring the user to close and re-open it.
		if !d.tickEnabled {
			d.tickEnabled = true
			return ActionCmd{Cmd: d.scheduleTick()}
		}

		// Confirmation mode for delete: only y/n/esc respond.
		if d.pendingDelete != "" {
			switch {
			case key.Matches(msg, d.keyMap.Confirm):
				id := d.pendingDelete
				d.pendingDelete = ""
				return ActionWeChatDelete{AccountID: id}
			case key.Matches(msg, d.keyMap.Cancel):
				d.pendingDelete = ""
			}
			return nil
		}
		switch {
		case key.Matches(msg, d.keyMap.Close):
			// Stop the tick loop before signaling close so any in-flight
			// tick cmd becomes a no-op when it eventually fires.
			d.stopTick()
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Up):
			if d.selected > 0 {
				d.selected--
			}
		case key.Matches(msg, d.keyMap.Down):
			if d.selected < len(d.accounts)-1 {
				d.selected++
			}
		case key.Matches(msg, d.keyMap.Select):
			if d.selected >= 0 && d.selected < len(d.accounts) {
				return ActionSelectWeChat{AccountID: d.accounts[d.selected].ID}
			}
		case key.Matches(msg, d.keyMap.Reconnect):
			if d.selected >= 0 && d.selected < len(d.accounts) {
				return ActionWeChatReconnect{AccountID: d.accounts[d.selected].ID}
			}
		case key.Matches(msg, d.keyMap.Toggle):
			if d.selected >= 0 && d.selected < len(d.accounts) {
				id := d.accounts[d.selected].ID
				mgr := wechat.GetManager()
				if mgr.IsRunning(id) {
					return ActionWeChatStop{AccountID: id}
				}
				return ActionWeChatStart{AccountID: id}
			}
		case key.Matches(msg, d.keyMap.Delete):
			if d.selected >= 0 && d.selected < len(d.accounts) {
				d.pendingDelete = d.accounts[d.selected].ID
			}
		case key.Matches(msg, d.keyMap.New):
			// Stop the tick while we're navigating away — the new dialog
			// will run in its own loop and we don't want two refreshers
			// stepping on each other.
			d.stopTick()
			return ActionOpenDialog{DialogID: WeChatQRID}
		}
	}
	return nil
}

// Draw implements Dialog.
func (d *WeChatManager) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	d.refreshAccounts()

	maxW := min(area.Dx()-4, 72)
	title := common.DialogTitle(t, "WeChat Manager", maxW,
		t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)

	// Header row.
	headerStyle := t.Dialog.ListItem.InfoBlurred
	header := headerStyle.Render(fmt.Sprintf("  %-10s  %-12s  %-10s  %s",
		"ACTIVE", "STATUS", "POLLING", "ACCOUNT (ID)"))

	// Account rows.
	var rows []string
	rows = append(rows, header, "")

	if len(d.accounts) == 0 {
		rows = append(rows, t.Dialog.ListItem.InfoBlurred.Render(
			"  No accounts. Press [n] to add a new one."))
	} else {
		for i, a := range d.accounts {
			style := t.Dialog.NormalItem
			marker := "  "
			if i == d.selected {
				style = t.Dialog.SelectedItem
				marker = "▶ "
			}
			activeMark := "  "
			if a.IsActive {
				activeMark = "● "
			}

			status := string(a.Status)
			if status == "" {
				status = "unknown"
			}
			poll := "off"
			if wechat.GetManager().IsRunning(a.ID) {
				poll = "running"
			}

			shortID := a.ID
			if len(shortID) > 10 {
				shortID = shortID[:10] + "…"
			}
			name := a.Name
			if name == "" {
				name = a.UserID
			}
			if len(name) > 18 {
				name = name[:18] + "…"
			}

			line := fmt.Sprintf("%s%s%-10s  %-12s  %-10s  %s (%s)",
				marker, activeMark, ternaryStr(a.IsActive, "active", ""),
				status, poll, name, shortID)
			rows = append(rows, style.Render(line))
		}
	}

	// Footer / context-sensitive help.
	var footer string
	if d.pendingDelete != "" {
		footer = t.Dialog.PrimaryText.Render(
			fmt.Sprintf("Delete account %q? This will remove all stored credentials and sessions.", d.pendingDelete))
	} else {
		footer = t.Dialog.ListItem.InfoBlurred.Render(strings.Join([]string{
			"[↑/k ↓/j move] [enter set-active] [r reconnect] [s start/stop]",
			"[d delete] [n new] [esc close]",
		}, "\n"))
	}

	view := lipgloss.JoinVertical(lipgloss.Left, title, "", header,
		lipgloss.JoinVertical(lipgloss.Left, rows...), "", footer)

	DrawCenter(scr, area, t.Dialog.View.Width(maxW).Render(view))
	return nil
}

func ternaryStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
