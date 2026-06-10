package worker

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/rabbitmq/amqp091-go"
	"github.com/resend/resend-go/v2"
)

// Shared event payload formats
type UserRegisteredEvent struct {
	Username         string `json:"username"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
}

type UserVerifiedEvent struct {
	Username string `json:"username"`
	Email    string `json:"email"`
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
	// 1. Declare the exchange here too so it dynamically constructs if not up yet
	err := w.amqpChan.ExchangeDeclare("auth.events", "topic", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Notification worker exchange declaration failed: %v", err)
	}

	// 2. Declare a single shared queue dedicated strictly to notifications
	q, err := w.amqpChan.QueueDeclare("notification.auth_events_queue", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Notification worker queue declaration failed: %v", err)
	}

	// 3. Bind the queue to both routing keys from the central exchange 🚀
	routingKeys := []string{"auth.user_registered", "auth.user_verified"}
	for _, key := range routingKeys {
		err = w.amqpChan.QueueBind(q.Name, key, "auth.events", false, nil)
		if err != nil {
			log.Fatalf("Critical: Failed to bind key %s to exchange: %v", key, err)
		}
	}

	// 4. Consume incoming data streams from the single bound queue
	msgs, err := w.amqpChan.Consume(
		q.Name, 
		"notification-service-consumer",  
		true, false, false, false, nil,                    
	)
	if err != nil {
		log.Fatalf("Critical: Notification worker subscription execution failed: %v", err)
	}

	go func() {
		log.Println("Notification Microservice Worker active via Topic Exchange. Polling... 🎧")
		for d := range msgs {
			// Routes logic seamlessly based on message headers
			switch d.RoutingKey {
			
			case "auth.user_registered":
				var event UserRegisteredEvent
				if err := json.Unmarshal(d.Body, &event); err == nil {
					log.Printf("[Event Caught] Routing verification code to: %s", event.Email)
					w.sendVerificationEmail(event)
				}

			case "auth.user_verified":
				var event UserVerifiedEvent
				if err := json.Unmarshal(d.Body, &event); err == nil {
					log.Printf("[Event Caught] Routing welcome onboarding layout to: %s", event.Email)
					w.sendWelcomeEmail(event)
				}
			}
		}
	}()
}

func (w *NotificationWorker) sendVerificationEmail(event UserRegisteredEvent) {
	htmlTemplate := fmt.Sprintf(`
		<div style="background-color: #0b0b0f; color: #ffffff; padding: 40px; font-family: sans-serif; text-align: center; border-radius: 8px;">
			<h1 style="color: #ff0055; font-size: 32px; letter-spacing: 2px;">PhonkDrift</h1>
			<p style="color: #8a8a93;">Welcome to the drift station, @%s.</p>
			<hr style="border: 0; border-top: 1px solid #1f1f29; margin: 30px 0;" />
			<p>Your 6-digit verification code is below. <strong>Note: This security access token will completely expire in exactly 15 minutes.</strong></p>
			<div style="background-color: #161622; border: 2px solid #ff0055; display: inline-block; padding: 15px 40px; font-size: 36px; font-weight: bold; color: #ffffff; border-radius: 4px;">
				%s
			</div>
		</div>
	`, event.Username, event.VerificationCode)

	params := &resend.SendEmailRequest{
		From:    "PhonkDrift Onboarding <onboarding@teamnest.me>", 
		To:      []string{event.Email},
		Subject: "Complete Your PhonkDrift Profile Setup",
		Html:    htmlTemplate,
	}

	_, err := w.resendClient.Emails.Send(params)
	if err != nil {
		log.Printf("Resend verification delivery failed: %v", err)
		return
	}
	log.Printf("Success: Drift verification email routed to inbox for user: %s ✓", event.Username)
}

func (w *NotificationWorker) sendWelcomeEmail(event UserVerifiedEvent) {
	htmlContent := fmt.Sprintf(`
		<div style="background-color: #0d0d0d; color: #ffffff; padding: 40px; font-family: sans-serif; text-align: center; border-radius: 8px;">
			<h1 style="color: #ff0055; font-size: 32px; letter-spacing: 2px;">WELCOME TO THE DRIFT, @%s! 🏎️💨</h1>
			<hr style="border: 0; border-top: 1px solid #1f1f29; margin: 30px 0;" />
			<p style="color: #8a8a93;">Your account is officially verified. Get ready to experience high-octane performance.</p>
		</div>
	`, event.Username)
		
	params := &resend.SendEmailRequest{
		From:    "PhonkDrift Onboarding <onboarding@teamnest.me>",
		To:      []string{event.Email},
		Subject: "Welcome to PhonkDrift!",
		Html:    htmlContent,
	}

	_, err := w.resendClient.Emails.Send(params)
	if err != nil {
		log.Printf("Resend welcome delivery failed: %v", err)
		return
	}
	log.Printf("Success: Welcome email sent to verified user: %s ✓", event.Username)
}