package handlers

import (
	"context"

	"cloud.google.com/go/pubsub"
)

// PubSubPublisher implements EventPublisher using Google Cloud Pub/Sub
type PubSubPublisher struct {
	client *pubsub.Client
}

// NewPubSubPublisher creates a new Pub/Sub-backed event publisher
func NewPubSubPublisher(client *pubsub.Client) *PubSubPublisher {
	return &PubSubPublisher{client: client}
}

func (p *PubSubPublisher) Publish(ctx context.Context, topic string, data []byte) error {
	t := p.client.Topic(topic)
	exists, err := t.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		t, err = p.client.CreateTopic(ctx, topic)
		if err != nil {
			return err
		}
	}

	result := t.Publish(ctx, &pubsub.Message{Data: data})
	_, err = result.Get(ctx)
	return err
}
