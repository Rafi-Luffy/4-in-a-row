package kafka

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer() (*Producer, error) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		// If no KAFKA_BROKERS is set, return nil to indicate no Kafka
		return nil, nil
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Topic:        "game-events",
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	producer := &Producer{writer: writer}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte("test"),
		Value: []byte("connection test"),
	})
	
	if err != nil {
		log.Printf("Kafka connection test failed: %v", err)
		return nil, err
	}

	log.Println("Kafka producer initialized successfully")
	return producer, nil
}

func (p *Producer) SendMessage(topic, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(time.Now().Format(time.RFC3339)),
		Value: []byte(message),
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}