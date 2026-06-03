package dialog

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/package-register/mocode/internal/ui/common"
	wechat "github.com/package-register/mocode/internal/wechat"
)

const (
	WeChatQRID = "wechat_qr"
)

// WeChatQRState is the QR login flow state.
type WeChatQRState int

const (
	WeChatQRStateGenerating WeChatQRState = iota
	WeChatQRStateDisplay
	WeChatQRStateScanned
	WeChatQRStateLoggedIn
	WeChatQRStateError
)

// WeChatQRMsg is sent when the QR login state changes.
type WeChatQRMsg struct {
	State   WeChatQRState
	QRASCII string
	Error   string
	UserID  string
}

type WeChatQRPollMsg struct{}

// WeChatQR represents a dialog for WeChat QR login.
type WeChatQR struct {
	com   *common.Common
	help  help.Model
	state WeChatQRState

	qrASCII string
	errMsg  string
	userID  string

	loginDone chan WeChatQRMsg
	cancelFn  context.CancelFunc
	started   bool
	client    *http.Client

	keyMap struct {
		Close   key.Binding
		Refresh key.Binding
	}
}

var _ Dialog = (*WeChatQR)(nil)

// NewWeChatQR creates a new WeChat QR login dialog.
func NewWeChatQR(com *common.Common) (*WeChatQR, error) {
	d := &WeChatQR{
		com:       com,
		state:     WeChatQRStateGenerating,
		loginDone: make(chan WeChatQRMsg, 5),
	}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	d.help = h

	d.keyMap.Close = CloseKey
	d.keyMap.Refresh = key.NewBinding(
		key.WithKeys("r", "enter"),
		key.WithHelp("r/enter", "retry/confirm"),
	)

	return d, nil
}

// StartLogin begins the login flow. Called externally with agent handler already set.
func (d *WeChatQR) StartLogin() {
	if d.started {
		return
	}
	d.started = true
	d.state = WeChatQRStateGenerating

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelFn = cancel
	wc := wechat.Default()

	go func() {
		defer cancel()

		err := wc.LoginWithCallbacks(ctx, true, wechat.LoginCallbacks{
			OnQRURL: func(qrURL string) {
				qr, genErr := wechat.GenerateQR(qrURL)
				if genErr != nil {
					d.sendLoginMsg(ctx, WeChatQRMsg{State: WeChatQRStateError, Error: genErr.Error()})
					return
				}
				d.sendLoginMsg(ctx, WeChatQRMsg{State: WeChatQRStateDisplay, QRASCII: qr.ASCII})
			},
			OnScanned: func() {
				d.sendLoginMsg(ctx, WeChatQRMsg{State: WeChatQRStateScanned})
			},
			OnExpired: func() {
				d.sendLoginMsg(ctx, WeChatQRMsg{State: WeChatQRStateGenerating})
			},
			OnLoggedIn: func(userID string) {
				d.sendLoginMsg(ctx, WeChatQRMsg{State: WeChatQRStateLoggedIn, UserID: userID})
			},
		}, d.client)
		if err != nil {
			select {
			case d.loginDone <- WeChatQRMsg{State: WeChatQRStateError, Error: err.Error()}:
			case <-ctx.Done():
			}
			return
		}

		// Start the message poll loop in background.
		// Run blocks until Stop() or context cancel.
		go func() {
			_ = wc.Run(context.Background())
		}()
	}()
}

func (d *WeChatQR) sendLoginMsg(ctx context.Context, msg WeChatQRMsg) {
	select {
	case d.loginDone <- msg:
	case <-ctx.Done():
	}
}

// PollLoginCmd returns a command that polls for login state changes.
func (d *WeChatQR) PollLoginCmd() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-d.loginDone:
			if !ok {
				return nil
			}
			return msg
		case <-time.After(250 * time.Millisecond):
			return WeChatQRPollMsg{}
		}
	}
}

func (d *WeChatQR) SetHTTPClient(client *http.Client) {
	d.client = client
}

// ID returns the dialog ID.
func (d *WeChatQR) ID() string {
	return WeChatQRID
}

// HandleMsg implements Dialog.
func (d *WeChatQR) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case WeChatQRMsg:
		d.state = msg.State
		if msg.QRASCII != "" {
			d.qrASCII = msg.QRASCII
		}
		d.errMsg = msg.Error
		d.userID = msg.UserID
		return ActionCmd{Cmd: d.PollLoginCmd()}

	case WeChatQRPollMsg:
		if d.state == WeChatQRStateLoggedIn || d.state == WeChatQRStateError {
			return nil
		}
		return ActionCmd{Cmd: d.PollLoginCmd()}

	case tea.KeyPressMsg:
		// In logged-in state, any key closes.
		if d.state == WeChatQRStateLoggedIn {
			if key.Matches(msg, d.keyMap.Close, d.keyMap.Refresh) {
				return ActionClose{}
			}
		}

		if key.Matches(msg, d.keyMap.Close) {
			d.cleanup()
			return ActionClose{}
		}
		if key.Matches(msg, d.keyMap.Refresh) {
			if d.state == WeChatQRStateError {
				d.started = false
				d.StartLogin()
			}
		}
	}
	return nil
}

func (d *WeChatQR) cleanup() {
	if d.cancelFn != nil && d.state != WeChatQRStateLoggedIn {
		d.cancelFn()
		d.cancelFn = nil
	}
}

// Draw implements Dialog.
func (d *WeChatQR) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles

	maxW := area.Dx() - t.Dialog.View.GetHorizontalFrameSize() - 4
	width := max(30, min(60, maxW))

	rc := NewRenderContext(t, width)

	switch d.state {
	case WeChatQRStateGenerating:
		rc.Title = "WeChat Login"
		rc.AddPart(t.Dialog.PrimaryText.Render("Generating QR code..."))

	case WeChatQRStateDisplay:
		rc.Title = "Scan with WeChat"
		rc.AddPart(t.Dialog.PrimaryText.Render("Scan this code in WeChat, then confirm on your phone."))
		rc.AddPart(renderQRPanel(d.qrASCII))

	case WeChatQRStateScanned:
		rc.Title = "Confirm in WeChat"
		rc.AddPart(t.Dialog.PrimaryText.Render("QR scanned. Confirm login in WeChat."))
		rc.AddPart(renderQRPanel(d.qrASCII))

	case WeChatQRStateLoggedIn:
		rc.Title = "WeChat Connected"
		rc.AddPart(t.Dialog.PrimaryText.Render(
			fmt.Sprintf("Logged in: %s\n\nPress Enter to close.\nSend messages from WeChat!", d.userID),
		))

	case WeChatQRStateError:
		rc.Title = "WeChat Error"
		rc.AddPart(t.Dialog.PrimaryText.Render(fmt.Sprintf("Error: %s\nPress r to retry.", d.errMsg)))
	}

	rc.Help = d.help.View(d)
	view := rc.Render()
	DrawCenterCursor(scr, area, view, nil)
	return nil
}

func renderQRPanel(qrASCII string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#FFFFFF")).
		Foreground(lipgloss.Color("#000000")).
		Padding(1, 2).
		Render(qrASCII)
}

// ShortHelp implements help.KeyMap.
func (d *WeChatQR) ShortHelp() []key.Binding {
	switch d.state {
	case WeChatQRStateLoggedIn:
		return []key.Binding{d.keyMap.Refresh}
	case WeChatQRStateError:
		return []key.Binding{d.keyMap.Refresh, d.keyMap.Close}
	default:
		return []key.Binding{d.keyMap.Close}
	}
}

// FullHelp implements help.KeyMap.
func (d *WeChatQR) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Close, d.keyMap.Refresh}}
}
