package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	url := os.Getenv("SENTINEL_NATS_URL")
	if url == "" {
		url = "nats://localhost:4222"
	}

	nc, err := nats.Connect(url)
	if err != nil {
		log.Fatalf("failed to connect to nats: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("failed to get jetstream context: %v", err)
	}

	ctx := context.Background()
	cons, err := js.CreateOrUpdateConsumer(ctx, "SENTINEL_AGENT", jetstream.ConsumerConfig{
		Durable:       "test-consumer",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "sentinel.agent.>",
	})
	if err != nil {
		log.Fatalf("failed to create consumer: %v", err)
	}

	log.Println("Consumer started, waiting for events...")

	iter, err := cons.Messages()
	if err != nil {
		log.Fatalf("failed to get message iterator: %v", err)
	}

	go func() {
		for {
			msg, err := iter.Next()
			if err != nil {
				// Iterator might close when connection is lost
				time.Sleep(1 * time.Second)
				continue
			}

			fmt.Printf("Received Event: %s (MsgID: %s)\n%s\n", 
				msg.Subject(), msg.Headers().Get("Nats-Msg-Id"), string(msg.Data()))
			
			// Acknowledge the message
			if err := msg.Ack(); err != nil {
				log.Printf("Failed to ack message: %v\n", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Consumer shutting down...")
}
