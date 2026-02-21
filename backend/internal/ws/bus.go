package ws

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const busChannel = "ws:events"

type Bus struct {
	redis      *redis.Client
	instanceID string
}

type busMessage struct {
	Source string      `json:"source"`
	RoomID string      `json:"room_id"`
	Event  ServerEvent `json:"event"`
}

func NewBus(redisClient *redis.Client, instanceID string) *Bus {
	return &Bus{redis: redisClient, instanceID: instanceID}
}

func (b *Bus) Publish(ctx context.Context, roomID string, event ServerEvent) error {
	msg := busMessage{
		Source: b.instanceID,
		RoomID: roomID,
		Event:  event,
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return b.redis.Publish(ctx, busChannel, raw).Err()
}

func (b *Bus) Subscribe(ctx context.Context, onEvent func(roomID string, event ServerEvent)) error {
	sub := b.redis.Subscribe(ctx, busChannel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var envelope busMessage
			if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
				continue
			}
			if envelope.Source == b.instanceID {
				continue
			}
			onEvent(envelope.RoomID, envelope.Event)
		}
	}
}
