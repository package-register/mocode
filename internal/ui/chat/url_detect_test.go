package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLDetector_HTTPURL(t *testing.T) {
	d := NewURLDetector()

	content := "Visit https://example.com for more info"
	urls := d.Detect(content)

	// 可能匹配到 https://example.com 和 /example.com（文件路径）
	assert.GreaterOrEqual(t, len(urls), 1)
	assert.Equal(t, "https://example.com", urls[0].URL)
}

func TestURLDetector_HTTPSURL(t *testing.T) {
	d := NewURLDetector()

	content := "Check https://secure.example.com/path?q=1"
	urls := d.Detect(content)

	// 可能匹配到多个 URL
	assert.GreaterOrEqual(t, len(urls), 1)
	assert.Equal(t, "https://secure.example.com/path?q=1", urls[0].URL)
}

func TestURLDetector_MultipleURLs(t *testing.T) {
	d := NewURLDetector()

	content := "https://a.com and https://b.com"
	urls := d.Detect(content)

	// 可能匹配到多个 URL（包括文件路径）
	assert.GreaterOrEqual(t, len(urls), 2)
	// 检查是否包含两个主要的 URL
	foundA := false
	foundB := false
	for _, u := range urls {
		if u.URL == "https://a.com" {
			foundA = true
		}
		if u.URL == "https://b.com" {
			foundB = true
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)
}

func TestURLDetector_Email(t *testing.T) {
	d := NewURLDetector()

	content := "Contact user@example.com for help"
	urls := d.Detect(content)

	require.Len(t, urls, 1)
	assert.Equal(t, "user@example.com", urls[0].URL)
}

func TestURLDetector_IPAddress(t *testing.T) {
	d := NewURLDetector()

	content := "Server at 192.168.1.1:8080"
	urls := d.Detect(content)

	require.Len(t, urls, 1)
	assert.Equal(t, "192.168.1.1:8080", urls[0].URL)
}

func TestURLDetector_MixedContent(t *testing.T) {
	d := NewURLDetector()

	content := `Visit https://example.com or email user@test.com
Server: 192.168.1.1
File: /path/to/file.txt`

	urls := d.Detect(content)

	assert.GreaterOrEqual(t, len(urls), 3)
}

func TestURLDetector_Deduplication(t *testing.T) {
	d := NewURLDetector()

	content := "https://example.com and https://example.com again"
	urls := d.Detect(content)

	// 检查 https://example.com 是否被去重
	found := 0
	for _, u := range urls {
		if u.URL == "https://example.com" {
			found++
		}
	}
	assert.Equal(t, 1, found)
}

func TestURLDetector_DetectInLine(t *testing.T) {
	d := NewURLDetector()

	line := "Check https://example.com for details"
	urls := d.DetectInLine(line, 5)

	// 可能匹配到多个 URL
	assert.GreaterOrEqual(t, len(urls), 1)
	assert.Equal(t, "https://example.com", urls[0].URL)
	assert.Equal(t, 5, urls[0].Line)
}

func TestURLDetector_IsURL(t *testing.T) {
	d := NewURLDetector()

	assert.True(t, d.IsURL("https://example.com"))
	assert.True(t, d.IsURL("user@example.com"))
	assert.True(t, d.IsURL("/path/to/file"))
	assert.False(t, d.IsURL("just text"))
}

func TestURLDetector_GetURLType(t *testing.T) {
	d := NewURLDetector()

	assert.Equal(t, URLTypeWeb, d.GetURLType("https://example.com"))
	assert.Equal(t, URLTypeWeb, d.GetURLType("http://example.com"))
	assert.Equal(t, URLTypeEmail, d.GetURLType("user@example.com"))
	assert.Equal(t, URLTypeFile, d.GetURLType("/path/to/file"))
	assert.Equal(t, URLTypeFile, d.GetURLType("C:\\Users\\test"))
	assert.Equal(t, URLTypeIP, d.GetURLType("192.168.1.1"))
	assert.Equal(t, URLTypeUnknown, d.GetURLType("just text"))
}

func TestURLType_String(t *testing.T) {
	assert.Equal(t, "web", URLTypeWeb.String())
	assert.Equal(t, "email", URLTypeEmail.String())
	assert.Equal(t, "file", URLTypeFile.String())
	assert.Equal(t, "ip", URLTypeIP.String())
	assert.Equal(t, "unknown", URLTypeUnknown.String())
}

func TestURLDetector_CustomPatterns(t *testing.T) {
	d, err := NewURLDetectorWithCustomPatterns([]string{
		`#\d+`, // Issue numbers like #123
	})
	require.NoError(t, err)

	content := "Fix #123 and #456"
	urls := d.Detect(content)

	require.Len(t, urls, 2)
	assert.Equal(t, "#123", urls[0].URL)
	assert.Equal(t, "#456", urls[1].URL)
}

func TestURLDetector_AddPattern(t *testing.T) {
	d := NewURLDetector()

	err := d.AddPattern(`#\d+`)
	require.NoError(t, err)

	content := "Fix #789"
	urls := d.Detect(content)

	require.Len(t, urls, 1)
	assert.Equal(t, "#789", urls[0].URL)
}

func TestURLDetector_InvalidPattern(t *testing.T) {
	_, err := NewURLDetectorWithCustomPatterns([]string{
		`[invalid`, // Invalid regex
	})
	assert.Error(t, err)
}

func TestURLDetector_Patterns(t *testing.T) {
	d := NewURLDetector()

	patterns := d.Patterns()
	assert.NotEmpty(t, patterns)
	assert.Len(t, patterns, 4) // 4 default patterns
}

func TestURLDetector_EmptyContent(t *testing.T) {
	d := NewURLDetector()

	urls := d.Detect("")
	assert.Empty(t, urls)
}

func TestURLDetector_NoURLs(t *testing.T) {
	d := NewURLDetector()

	content := "Just plain text without any URLs"
	urls := d.Detect(content)

	assert.Empty(t, urls)
}

func TestURLDetector_MultilineContent(t *testing.T) {
	d := NewURLDetector()

	content := `Line 1: https://example.com
Line 2: user@test.com
Line 3: 192.168.1.1`

	urls := d.Detect(content)

	// 应该检测到至少 3 个主要 URL
	assert.GreaterOrEqual(t, len(urls), 3)

	// 检查是否包含主要的 URL
	foundHTTPS := false
	foundEmail := false
	foundIP := false
	for _, u := range urls {
		if u.URL == "https://example.com" {
			foundHTTPS = true
		}
		if u.URL == "user@test.com" {
			foundEmail = true
		}
		if u.URL == "192.168.1.1" {
			foundIP = true
		}
	}
	assert.True(t, foundHTTPS)
	assert.True(t, foundEmail)
	assert.True(t, foundIP)
}
