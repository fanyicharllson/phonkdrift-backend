package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// TrackDiscoveredEvent tells discovery-service a track was just persisted from
// a user favorite/playlist-add and needs its audio downloaded + uploaded to
// DO Spaces in the background.
type TrackDiscoveredEvent struct {
	TrackID   string    `json:"track_id"`
	YoutubeID string    `json:"youtube_id"`
	Timestamp time.Time `json:"timestamp"`
}

// TrackEventPublisher is a concrete type (not an interface) to match
// track-service's existing non-hexagonal style of taking concrete
// dependencies (db.Querier, *redis.Client) directly.
type TrackEventPublisher struct {
	amqpChan *amqp091.Channel
}

func NewTrackEventPublisher(ch *amqp091.Channel) *TrackEventPublisher {
	return &TrackEventPublisher{amqpChan: ch}
}

func (p *TrackEventPublisher) PublishTrackDiscovered(ctx context.Context, trackID, youtubeID string) error {
	event := TrackDiscoveredEvent{
		TrackID:   trackID,
		YoutubeID: youtubeID,
		Timestamp: time.Now(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := p.amqpChan.ExchangeDeclare("track.events", "topic", true, false, false, false, nil); err != nil {
		return err
	}

	return p.amqpChan.PublishWithContext(ctx,
		"track.events",
		"track.discovered",
		false,
		false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Body:         body,
		},
	)
}
