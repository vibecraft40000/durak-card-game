package engine

import "time"

func Expired(state GameState, now time.Time) bool {
	if state.Status != StatusPlaying {
		return false
	}
	return now.After(state.TurnEndsAt)
}
