package ws

import (
	"testing"

	"durakonline/backend/internal/rooms"
)

func TestShouldPreserveRoomMembershipOnDisconnect(t *testing.T) {
	cases := map[rooms.Status]bool{
		rooms.StatusWaiting:              true,
		rooms.StatusConfirmed:            true,
		rooms.StatusAwaitingStakeConfirm: true,
		rooms.StatusFinished:             true,
		rooms.StatusCancelled:            true,
		rooms.StatusInGame:               false,
	}

	for status, want := range cases {
		if got := shouldPreserveRoomMembershipOnDisconnect(status); got != want {
			t.Fatalf("shouldPreserveRoomMembershipOnDisconnect(%q) = %v, want %v", status, got, want)
		}
	}
}
