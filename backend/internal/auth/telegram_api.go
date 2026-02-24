package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
