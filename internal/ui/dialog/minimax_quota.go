package dialog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/package-register/mocode/internal/minimax"
	"github.com/package-register/mocode/internal/ui/common"
	"github.com/package-register/mocode/internal/ui/styles"
)

const (
	MiniMaxQuotaID          = "minimax_quota"
	miniMaxQuotaDialogWidth = 72
)

// MiniMaxQuotaState represents the fetching state.
type MiniMaxQuotaState int

const (
	MiniMaxQuotaStateLoading MiniMaxQuotaState = iota
	MiniMaxQuotaStateLoaded
	MiniMaxQuotaStateError
)

// miniMaxQuotaMsg carries a quota fetch result.
type miniMaxQuotaMsg struct {
	Err    error
	Tables []string // one table per provider
}

// MiniMaxQuota is a dialog that displays MiniMax API quota usage.
type MiniMaxQuota struct {
	com   *common.Common
	help  help.Model
	state MiniMaxQuotaState

	tables []string
	errMsg string

	resultCh chan miniMaxQuotaMsg
	cancelFn context.CancelFunc

	keyMap struct {
		Close   key.Binding
		Refresh key.Binding
	}
}

var _ Dialog = (*MiniMaxQuota)(nil)

// NewMiniMaxQuota creates a new MiniMax quota dialog and starts fetching.
func NewMiniMaxQuota(com *common.Common) (*MiniMaxQuota, error) {
	d := &MiniMaxQuota{
		com:      com,
		state:    MiniMaxQuotaStateLoading,
		resultCh: make(chan miniMaxQuotaMsg, 1),
	}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	d.help = h

	d.keyMap.Close = CloseKey
	d.keyMap.Refresh = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	)

	d.startFetch()

	return d, nil
}

func (d *MiniMaxQuota) startFetch() {
	ctx, cancel := context.WithCancel(context.Background())
	if d.cancelFn != nil {
		d.cancelFn()
	}
	d.cancelFn = cancel
	d.state = MiniMaxQuotaStateLoading
	d.tables = nil
	d.errMsg = ""

	go func() {
		defer cancel()

		cfg := d.com.Config()
		if cfg == nil || !cfg.IsConfigured() {
			select {
			case d.resultCh <- miniMaxQuotaMsg{Err: fmt.Errorf("no providers configured")}:
			case <-ctx.Done():
			}
			return
		}

		var tables []string
		found := false
		for providerID, provider := range cfg.Providers.Seq2() {
			if provider.Disable {
				continue
			}
			if !minimax.IsProvider(providerID, provider) {
				continue
			}

			regionBaseURL := minimax.QuotaBaseURL(provider)

			apiKey := strings.TrimSpace(provider.APIKey)
			if apiKey == "" {
				continue
			}

			fetchCtx, fetchCancel := context.WithTimeout(ctx, 12*time.Second)
			client := minimax.NewMiniMaxQuotaClient(apiKey, regionBaseURL)
			client.SetHTTPClient(d.com.Config().HTTPClient(d.com.Workspace.Resolver(), 12*time.Second))
			// Always set cookie if available (same behavior as Admin UI).
			if value := minimax.QuotaCookie(provider); value != "" {
				client.SetCookie(value)
			}
			resp, err := client.FetchQuota(fetchCtx)
			fetchCancel()
			if err != nil {
				select {
				case d.resultCh <- miniMaxQuotaMsg{Err: fmt.Errorf("provider %q: %w", providerID, err)}:
				case <-ctx.Done():
				}
				return
			}

			header := fmt.Sprintf("Provider: %s (%s)", providerID, regionBaseURL)
			tables = append(tables, header+"\n"+minimax.FormatQuotaTable(resp.ModelRemain))
			found = true
		}

		if !found {
			select {
			case d.resultCh <- miniMaxQuotaMsg{Err: fmt.Errorf("no MiniMax providers found")}:
			case <-ctx.Done():
			}
			return
		}

		select {
		case d.resultCh <- miniMaxQuotaMsg{Tables: tables}:
		case <-ctx.Done():
		}
	}()
}

// FetchQuotaCmd returns a command that waits for the quota result.
// It blocks on the channel (bubbletea runs each Cmd in its own goroutine)
// and returns the result when the background fetch completes.
func (d *MiniMaxQuota) FetchQuotaCmd() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-d.resultCh
		if !ok {
			return nil
		}
		return msg
	}
}

// ID returns the dialog ID.
func (d *MiniMaxQuota) ID() string {
	return MiniMaxQuotaID
}

// HandleMsg implements Dialog.
func (d *MiniMaxQuota) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case miniMaxQuotaMsg:
		if msg.Err != nil {
			d.state = MiniMaxQuotaStateError
			d.errMsg = msg.Err.Error()
		} else {
			d.state = MiniMaxQuotaStateLoaded
			d.tables = msg.Tables
		}
		return nil

	case tea.KeyPressMsg:
		if key.Matches(msg, d.keyMap.Close) {
			d.cleanup()
			return ActionClose{}
		}
		if key.Matches(msg, d.keyMap.Refresh) {
			d.startFetch()
		}
	}
	return nil
}

func (d *MiniMaxQuota) cleanup() {
	if d.cancelFn != nil {
		d.cancelFn()
		d.cancelFn = nil
	}
}

// Draw implements Dialog.
func (d *MiniMaxQuota) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles

	maxW := area.Dx() - t.Dialog.View.GetHorizontalFrameSize() - 4
	width := max(40, min(miniMaxQuotaDialogWidth, maxW))

	rc := NewRenderContext(t, width)
	rc.Title = "MiniMax Quota"

	switch d.state {
	case MiniMaxQuotaStateLoading:
		rc.AddPart(t.Dialog.PrimaryText.Render("Fetching MiniMax quota..."))

	case MiniMaxQuotaStateLoaded:
		rc.AddPart(renderQuotaContent(t, d.tables))

	case MiniMaxQuotaStateError:
		rc.AddPart(t.Dialog.PrimaryText.Render(
			fmt.Sprintf("Error: %s\n\nPress r to retry.", d.errMsg),
		))
	}

	rc.Help = d.help.View(d)
	view := rc.Render()
	DrawCenterCursor(scr, area, view, nil)
	return nil
}

func renderQuotaContent(t *styles.Styles, tables []string) string {
	content := strings.Join(tables, "\n\n")
	return t.Dialog.PrimaryText.Width(miniMaxQuotaDialogWidth - 8).Render(content)
}

// ShortHelp implements help.KeyMap.
func (d *MiniMaxQuota) ShortHelp() []key.Binding {
	switch d.state {
	case MiniMaxQuotaStateError:
		return []key.Binding{d.keyMap.Refresh, d.keyMap.Close}
	default:
		return []key.Binding{d.keyMap.Close}
	}
}

// FullHelp implements help.KeyMap.
func (d *MiniMaxQuota) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Close, d.keyMap.Refresh}}
}
