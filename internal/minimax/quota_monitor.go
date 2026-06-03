package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// MiniMaxQuotaResponse MiniMax coding_plan/remains API 响应
type MiniMaxQuotaResponse struct {
	BaseResp    MiniMaxBaseResp      `json:"base_resp"`
	ModelRemain []MiniMaxModelRemain `json:"model_remains"`
}

// MiniMaxBaseResp 基础响应状态
type MiniMaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// MiniMaxModelRemain 单个模型的完整配额信息
type MiniMaxModelRemain struct {
	ModelName                 string `json:"model_name"`
	StartTime                 int64  `json:"start_time"`                   // 当前周期起始 Unix 秒
	EndTime                   int64  `json:"end_time"`                     // 当前周期结束 Unix 秒
	RemainsTime               int64  `json:"remains_time"`                 // 距周期重置剩余秒数
	CurrentIntervalTotalCount int64  `json:"current_interval_total_count"` // 周期总配额
	CurrentIntervalUsageCount int64  `json:"current_interval_usage_count"` // 周期已用次数
	CurrentWeeklyTotalCount   int64  `json:"current_weekly_total_count"`   // 周总配额
	CurrentWeeklyUsageCount   int64  `json:"current_weekly_usage_count"`   // 周已用次数
	WeeklyStartTime           int64  `json:"weekly_start_time"`            // 周起始 Unix 秒
	WeeklyEndTime             int64  `json:"weekly_end_time"`              // 周结束 Unix 秒
	WeeklyRemainsTime         int64  `json:"weekly_remains_time"`          // 距周重置剩余秒数
}

// Used 返回当前周期已用量
func (m MiniMaxModelRemain) Used() int64 {
	return m.CurrentIntervalTotalCount - m.CurrentIntervalUsageCount
}

// WeekUsed 返回当前周已用量
func (m MiniMaxModelRemain) WeekUsed() int64 {
	return m.CurrentWeeklyTotalCount - m.CurrentWeeklyUsageCount
}

// Percent 返回当前周期使用百分比
func (m MiniMaxModelRemain) Percent() float64 {
	if m.CurrentIntervalTotalCount <= 0 {
		return 0
	}
	return float64(m.Used()) / float64(m.CurrentIntervalTotalCount) * 100
}

// WeekPercent 返回当前周使用百分比
func (m MiniMaxModelRemain) WeekPercent() float64 {
	if m.CurrentWeeklyTotalCount <= 0 {
		return 0
	}
	return float64(m.WeekUsed()) / float64(m.CurrentWeeklyTotalCount) * 100
}

// ResetsIn 返回距离周期重置的时长
func (m MiniMaxModelRemain) ResetsIn() time.Duration {
	return time.Duration(m.RemainsTime) * time.Second
}

// WeekResetsIn 返回距离周重置的时长
func (m MiniMaxModelRemain) WeekResetsIn() time.Duration {
	return time.Duration(m.WeeklyRemainsTime) * time.Second
}

// MiniMaxQuotaClient 查询 MiniMax 配额的 HTTP 客户端
type MiniMaxQuotaClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewMiniMaxQuotaClient 创建配额查询客户端
func NewMiniMaxQuotaClient(apiKey, regionBaseURL string) *MiniMaxQuotaClient {
	host := strings.TrimSpace(regionBaseURL)
	if host == "" {
		host = "https://api.minimaxi.com"
	}
	return &MiniMaxQuotaClient{
		apiKey:  apiKey,
		baseURL: host,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *MiniMaxQuotaClient) SetCookie(cookie string) {
	cookie = strings.TrimSpace(cookie)
	if cookie != "" {
		next := c.httpClient.Transport
		if next == nil {
			next = http.DefaultTransport
		}
		c.httpClient.Transport = cookieRoundTripper{
			cookie: cookie,
			next:   next,
		}
	}
}

func (c *MiniMaxQuotaClient) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

// FetchQuota 拉取 MiniMax 配额用量，使用 Bearer token 认证（对齐官方 mmx CLI）。
func (c *MiniMaxQuotaClient) FetchQuota(ctx context.Context) (*MiniMaxQuotaResponse, error) {
	url := strings.TrimRight(c.baseURL, "/") + "/v1/api/openplatform/coding_plan/remains"

	resp, err := c.doQuotaRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	if resp.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("quota API error (code=%d): %s\nCheck that your API key is valid and belongs to a Token Plan: https://platform.minimaxi.com/subscribe/token-plan",
			resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
	}

	return resp, nil
}

// doQuotaRequest performs the quota HTTP request using Bearer token authentication.
func (c *MiniMaxQuotaClient) doQuotaRequest(ctx context.Context, url string) (*MiniMaxQuotaResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create quota request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("quota request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read quota response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quota request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var quotaResp MiniMaxQuotaResponse
	if err := json.Unmarshal(body, &quotaResp); err != nil {
		return nil, fmt.Errorf("parse quota response: %w", err)
	}

	return &quotaResp, nil
}

type cookieRoundTripper struct {
	cookie string
	next   http.RoundTripper
}

func (c cookieRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Cookie", c.cookie)
	return c.next.RoundTrip(cloned)
}

// FormatQuotaTable 格式化完整配额表格输出
func FormatQuotaTable(models []MiniMaxModelRemain) string {
	if len(models) == 0 {
		return "  (no quota data)\n"
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("60")).
		Padding(0, 1).
		MarginRight(1).
		Width(36)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	cards := make([]string, 0, len(models))
	for _, m := range models {
		interval := quotaLine("Interval", m.Used(), m.CurrentIntervalTotalCount, m.Percent(), okStyle, warnStyle)
		weekly := quotaLine("Weekly", m.WeekUsed(), m.CurrentWeeklyTotalCount, m.WeekPercent(), okStyle, warnStyle)
		period := fmt.Sprintf("%s → %s", formatUnixTime(m.StartTime), formatUnixTime(m.EndTime))
		reset := fmt.Sprintf("reset %s / week %s", formatDuration(m.ResetsIn()), formatDuration(m.WeekResetsIn()))
		body := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render(m.ModelName),
			"",
			interval,
			weekly,
			"",
			labelStyle.Render("period ")+valueStyle.Render(period),
			labelStyle.Render(reset),
		)
		cards = append(cards, cardStyle.Render(body))
	}

	var rows []string
	for i := 0; i < len(cards); i += 2 {
		if i+1 < len(cards) {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i], cards[i+1]))
		} else {
			rows = append(rows, cards[i])
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...) + "\n"
}

func quotaLine(label string, used, total int64, pct float64, okStyle, warnStyle lipgloss.Style) string {
	style := okStyle
	if pct >= 80 {
		style = warnStyle
	}
	return fmt.Sprintf("%-8s %s %d/%d %.1f%%",
		label,
		style.Render(renderBar(pct, 12)),
		used,
		total,
		pct,
	)
}

func renderBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	filled = min(filled, width)
	filled = max(filled, 0)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func formatUnixTime(ts int64) string {
	if ts <= 0 {
		return "N/A"
	}
	return time.Unix(ts, 0).Format("01-02 15:04")
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	if d <= 0 {
		return "now"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 24 {
		days := h / 24
		h = h % 24
		return fmt.Sprintf("%dd%dh", days, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
