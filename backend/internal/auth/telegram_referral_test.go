package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestExtractReferralCodeFromStartParam(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "room param", input: "room_123", want: ""},
		{name: "ref underscore", input: "ref_ABC123", want: "abc123"},
		{name: "ref dash", input: "ref-my_code", want: "my_code"},
		{name: "invalid chars", input: "ref_abc$123", want: ""},
		{name: "too short", input: "ref_ab", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractReferralCodeFromStartParam(tc.input)
			if got != tc.want {
				t.Fatalf("unexpected referral code: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestValidateInitData_ExtractsReferralCode(t *testing.T) {
	botToken := "test-bot-token"
	now := time.Now().UTC()

	userRaw, _ := json.Marshal(TelegramUser{
		ID:        1234567,
		Username:  "tester",
		FirstName: "Test",
		LastName:  "User",
	})

	values := url.Values{}
	values.Set("auth_date", strconv.FormatInt(now.Unix(), 10))
	values.Set("user", string(userRaw))
	values.Set("start_param", "ref_MyCode-1")

	dataCheckString := buildDataCheckString(values)
	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretMac.Write([]byte(botToken))
	secretKey := secretMac.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))

	user, _, referralCode, err := ValidateInitData(values.Encode(), botToken, false, 5*time.Minute, now)
	if err != nil {
		t.Fatalf("ValidateInitData returned error: %v", err)
	}
	if user.ID != 1234567 {
		t.Fatalf("unexpected user id: %d", user.ID)
	}
	if referralCode != "mycode-1" {
		t.Fatalf("unexpected referral code: %q", referralCode)
	}
}
