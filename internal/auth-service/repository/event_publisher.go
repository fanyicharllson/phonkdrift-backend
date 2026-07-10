package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
	"github.com/rabbitmq/amqp091-go"
)

type eventPublisher struct {
	amqpChan *amqp091.Channel
}

// UserRegisteredEvent defines the explicit JSON structure passing through RabbitMQ
type UserRegisteredEvent struct {
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	VerificationCode string    `json:"verification_code"`
	Timestamp        time.Time `json:"timestamp"`
}

func NewEventPublisher(ch *amqp091.Channel) domain.EventPublisher {
	return &eventPublisher{amqpChan: ch}
}

func (p *eventPublisher) PublishUserRegistered(ctx context.Context, username, email, verificationCode string) error {
	event := UserRegisteredEvent{
		Username:         username,
		Email:            email,
		VerificationCode: verificationCode,
		Timestamp:        time.Now(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// CHANGED FROM "" TO "auth.events" HERE:
	return p.amqpChan.PublishWithContext(ctx,
		"auth.events",          // Exchange matching our consumer 🚀
		"auth.user_registered", // Routing key
		false,                  
		false,                  
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, 
			Body:         body,
		},
	)
}

func (p *eventPublisher) PublishUserVerified(ctx context.Context, username, email string) error {
	payload := map[string]string{
		"username": username,
		"email":    email,
	}

	body, _ := json.Marshal(payload)

	return p.amqpChan.PublishWithContext(ctx,
		"auth.events",          // Exchange name
		"auth.user_verified",   // Routing key 🚀
		false,
		false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// AvatarUpdatedEvent carries the pre-uploaded CDN URL so the avatar worker only
// needs to persist it to the DB and notify the user — no binary data crosses RabbitMQ.
type AvatarUpdatedEvent struct {
	UserID    string    `json:"user_id"`
	AvatarURL string    `json:"avatar_url"`
	Timestamp time.Time `json:"timestamp"`
}

func (p *eventPublisher) PublishAvatarUpdated(ctx context.Context, userID, avatarURL string) error {
	event := AvatarUpdatedEvent{
		UserID:    userID,
		AvatarURL: avatarURL,
		Timestamp: time.Now(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := p.amqpChan.ExchangeDeclare("profile.events", "topic", true, false, false, false, nil); err != nil {
		return err
	}

	return p.amqpChan.PublishWithContext(ctx,
		"profile.events",
		"profile.avatar_updated",
		false,
		false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Body:         body,
		},
	)
}