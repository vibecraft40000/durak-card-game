package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// FetchUserPhotoURL returns the Telegram user's profile photo URL via Bot API.
// Returns empty string if no photo or API error.
func FetchUserPhotoURL(ctx context.Context, botToken string, telegramID int64) string {
	if botToken == "" || botToken == "dev-bot-token" {
		return ""
	}
	base := "https://api.telegram.org/bot" + botToken

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		base+"/getUserProfilePhotos?user_id="+fmt.Sprint(telegramID)+"&limit=1", nil)
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			TotalCount int `json:"total_count"`
			Photos     [][]struct {
				FileID   string `json:"file_id"`
				FileSize int    `json:"file_size"`
				Width    int    `json:"width"`
				Height   int    `json:"height"`
			} `json:"photos"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.OK {
		return ""
	}
	if out.Result.TotalCount == 0 || len(out.Result.Photos) == 0 || len(out.Result.Photos[0]) == 0 {
		return ""
	}
	fileID := out.Result.Photos[0][len(out.Result.Photos[0])-1].FileID

	req2, err := http.NewRequestWithContext(ctx, http.MethodGet,
		base+"/getFile?file_id="+url.QueryEscape(fileID), nil)
	if err != nil {
		return ""
	}
	resp2, err := client.Do(req2)
	if err != nil {
		return ""
	}
	defer resp2.Body.Close()

	var out2 struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&out2); err != nil || !out2.OK || out2.Result.FilePath == "" {
		return ""
	}
	return "https://api.telegram.org/file/bot" + botToken + "/" + out2.Result.FilePath
}

// CheckChannelMembership returns true if the user is a member (or admin/creator) of the given channel.
// Bot must be an administrator in the channel. channelID is the Telegram chat id (e.g. "-1001234567890").
// Returns false on API errors or if user is left/kicked.
func CheckChannelMembership(ctx context.Context, botToken string, channelID string, telegramUserID int64) bool {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return true
	}
	if botToken == "" || botToken == "dev-bot-token" {
		return true
	}
	base := "https://api.telegram.org/bot" + botToken
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		base+"/getChatMember?chat_id="+url.QueryEscape(channelID)+"&user_id="+strconv.FormatInt(telegramUserID, 10), nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.OK {
		return false
	}
	switch strings.ToLower(out.Result.Status) {
	case "creator", "administrator", "member", "restricted":
		return true
	default:
		return false
	}
}
