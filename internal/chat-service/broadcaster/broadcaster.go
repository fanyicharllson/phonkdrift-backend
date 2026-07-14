package broadcaster

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/domain"
	"github.com/rabbitmq/amqp091-go"
)

const exchangeName = "chat.messages"

// Broadcaster relays new chat messages published to a RabbitMQ fanout
// exchange out to every locally-registered SubscribeToChat stream on this
// pod. This is what makes realtime delivery work correctly across multiple
// chat-service replicas — a message sent to replica A still reaches clients
// streaming from replica B, since every replica has its own queue bound to
// the same fanout exchange.
type Broadcaster struct {
	amqpChan *amqp091.Channel

	mu          sync.Mutex
	subscribers map[chan *domain.ChatMessage]struct{}
}

func New(ch *amqp091.Channel) *Broadcaster {
	return &Broadcaster{
		amqpChan:    ch,
		subscribers: make(map[chan *domain.ChatMessage]struct{}),
	}
}

// Publish sends a newly created message to the fanout exchange so every
// chat-service replica (including this one) picks it up and forwards it to
// its locally-connected subscribers.
func (b *Broadcaster) Publish(ctx context.Context, msg *domain.ChatMessage) error {
	if err := b.amqpChan.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil); err != nil {
		return err
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return b.amqpChan.PublishWithContext(ctx, exchangeName, "", false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

// Start declares this replica's own exclusive, auto-delete queue bound to the
// fanout exchange and begins relaying incoming messages to every locally
// registered subscriber channel. Non-blocking — spawns its own goroutine.
func (b *Broadcaster) Start(ctx context.Context) error {
	if err := b.amqpChan.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil); err != nil {
		return err
	}

	q, err := b.amqpChan.QueueDeclare("", false, true, true, false, nil) // anonymous, auto-delete, exclusive
	if err != nil {
		return err
	}

	if err := b.amqpChan.QueueBind(q.Name, "", exchangeName, false, nil); err != nil {
		return err
	}

	msgs, err := b.amqpChan.Consume(q.Name, "", true, true, false, false, nil) // auto-ack, exclusive consumer
	if err != nil {
		return err
	}

	go func() {
		log.Println("💬 Chat broadcaster relaying messages... 🎧")
		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					return
				}
				var msg domain.ChatMessage
				if err := json.Unmarshal(d.Body, &msg); err != nil {
					log.Printf("⚠️ Failed to decode relayed chat message: %v", err)
					continue
				}
				b.fanOut(&msg)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Subscribe registers a new channel that receives every message relayed from
// this point forward. Callers must call Unsubscribe when the client
// disconnects (typically via `defer`).
func (b *Broadcaster) Subscribe() chan *domain.ChatMessage {
	ch := make(chan *domain.ChatMessage, 16)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan *domain.ChatMessage) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broadcaster) fanOut(msg *domain.ChatMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- msg:
		default:
			// Slow subscriber — drop rather than block the whole broadcaster.
		}
	}
}
