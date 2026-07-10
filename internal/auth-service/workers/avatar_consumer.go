package workers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/usecase"
	"github.com/rabbitmq/amqp091-go"
)

// AvatarUpdatedEvent mirrors repository.AvatarUpdatedEvent — kept as a plain
// struct here so this package doesn't need to import the repository package
// just to unmarshal a JSON payload it already knows the shape of.
type AvatarUpdatedEvent struct {
	UserID    string `json:"user_id"`
	AvatarURL string `json:"avatar_url"`
}

// AvatarWorker consumes profile.avatar_updated events, persists the CDN URL
// to the DB, and pushes an FCM notification telling the client to refresh.
type AvatarWorker struct {
	amqpChan *amqp091.Channel
	repo     domain.AuthRepository
}

func NewAvatarWorker(ch *amqp091.Channel, repo domain.AuthRepository) *AvatarWorker {
	return &AvatarWorker{amqpChan: ch, repo: repo}
}

func (w *AvatarWorker) Start() {
	if err := w.amqpChan.ExchangeDeclare("profile.events", "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("Critical: Avatar worker exchange declaration failed: %v", err)
	}

	q, err := w.amqpChan.QueueDeclare("auth.profile_events_queue", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Avatar worker queue declaration failed: %v", err)
	}

	if err := w.amqpChan.QueueBind(q.Name, "profile.avatar_updated", "profile.events", false, nil); err != nil {
		log.Fatalf("Critical: Failed to bind avatar worker queue: %v", err)
	}

	msgs, err := w.amqpChan.Consume(q.Name, "auth-service-avatar-consumer", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Avatar worker subscription failed: %v", err)
	}

	go func() {
		log.Println("Avatar Upload Worker active via Topic Exchange. Polling... 🖼️")
		for d := range msgs {
			var event AvatarUpdatedEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				log.Printf("⚠️ Failed to decode avatar_updated event: %v", err)
				continue
			}
			w.handleAvatarUpdated(event)
		}
	}()
}

func (w *AvatarWorker) handleAvatarUpdated(event AvatarUpdatedEvent) {
	ctx := context.Background()

	user, err := w.repo.UpdateAvatarURL(ctx, event.UserID, event.AvatarURL)
	if err != nil || user == nil {
		log.Printf("⚠️ Failed to persist avatar_url for user %s: %v", event.UserID, err)
		return
	}
	log.Printf("✅ avatar_url updated for user %s", event.UserID)

	if user.FCMToken == "" {
		return
	}
	if _, err := usecase.SendFCMNotifications(
		[]string{user.FCMToken},
		"Profile updated",
		"Your new avatar is live!",
		"profile_updated",
		event.UserID,
	); err != nil {
		log.Printf("⚠️ Failed to send avatar-updated push to user %s: %v", event.UserID, err)
	}
}
