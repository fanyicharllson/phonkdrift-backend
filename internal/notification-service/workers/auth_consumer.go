package worker

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/rabbitmq/amqp091-go"
	"github.com/resend/resend-go/v2"
)

// Shared event payload format matching the Auth Service producer contract
type UserRegisteredEvent struct {
	Username         string `json:"username"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
}

type NotificationWorker struct {
	amqpChan     *amqp091.Channel
	resendClient *resend.Client
}

func NewNotificationWorker(ch *amqp091.Channel, resendKey string) *NotificationWorker {
	return &NotificationWorker{
		amqpChan:     ch,
		resendClient: resend.NewClient(resendKey),
	}
}

func (w *NotificationWorker) Start() {
	msgs, err := w.amqpChan.Consume(
		"auth.user_registered", 
		"notification-service-consumer",  
		true,                   
		false,                  
		false,                  
		false,                  
		nil,                    
	)
	if err != nil {
		log.Fatalf("Critical: Notification worker failed to subscribe: %v", err)
	}

	go func() {
		log.Println("Notification Microservice Worker active. Polling incoming broker streams... 🎧")
		for d := range msgs {
			var event UserRegisteredEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				log.Printf("Worker payload parse error: %v", err)
				continue
			}

			log.Printf("[Event Caught] Routing verification template to: %s", event.Email)
			w.sendEmail(event)
		}
	}()
}

func (w *NotificationWorker) sendEmail(event UserRegisteredEvent) {
	htmlTemplate := fmt.Sprintf(`
		<div style="background-color: #0b0b0f; color: #ffffff; padding: 40px; font-family: sans-serif; text-align: center; border-radius: 8px;">
			<h1 style="color: #ff0055; font-size: 32px; letter-spacing: 2px;">PhonkDrift</h1>
			<p style="color: #8a8a93;">Welcome to the drift station, @%s.</p>
			<hr style="border: 0; border-top: 1px solid #1f1f29; margin: 30px 0;" />
			<p>Your 6-digit verification code is:</p>
			<div style="background-color: #161622; border: 2px solid #ff0055; display: inline-block; padding: 15px 40px; font-size: 36px; font-weight: bold; color: #ffffff; border-radius: 4px;">
				%s
			</div>
		</div>
	`, event.Username, event.VerificationCode)

	params := &resend.SendEmailRequest{
		From:    "PhonkDrift Onboarding <onboarding@teamnest.me>", // Swap out with your domain
		To:      []string{event.Email},
		Subject: "Complete Your PhonkDrift Profile Setup",
		Html:    htmlTemplate,
	}

	_, err := w.resendClient.Emails.Send(params)
	if err != nil {
		log.Printf("Resend delivery failed: %v", err)
		return
	}
	log.Printf("Success: Drift verification email routed to inbox for user: %s ✓", event.Username)
}