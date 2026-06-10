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

	return p.amqpChan.PublishWithContext(ctx,
		"",                     // Exchange
		"auth.user_registered", // Routing key / Queue name
		false,                  // Mandatory
		false,                  // Immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, // Keeps message safe if broker restarts
			Body:         body,
		},
	)
}