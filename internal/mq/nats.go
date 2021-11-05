package mq

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

//go:generate mockgen -destination=mock/cache_mock.go -package=mock . MQ

// MQ defines interface for message queue
type MQ interface {
	Subscribe(topic string, hf HandleFunc) error
	Publish(topic string, msg []byte) error
}

// Config message queue configuratino
type Config struct {
	ServerURL string `yaml:"server_url"`
}

// HandleFunc message queue subscription handler
type HandleFunc = func([]byte) error

// Nats implements MQ
type Nats struct {
	Conn *nats.Conn
	ctx  context.Context
	cfg  *Config
	lg   *zap.Logger
}

// NewNats constructor
func NewNats(ctx context.Context, cfg *Config, lg *zap.Logger) (*Nats, error) {
	conn, err := nats.Connect(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("connect to nats server: %w", err)
	}

	return &Nats{
		Conn: conn,
		ctx:  ctx,
		cfg:  cfg,
		lg:   lg,
	}, nil
}

// Subscribe handles incoming from topic messages
func (n *Nats) Subscribe(topic string, hf HandleFunc) error {
	ch := make(chan *nats.Msg, nats.DefaultSubPendingMsgsLimit)

	sub, err := n.Conn.ChanSubscribe(topic, ch)
	if err != nil {
		return fmt.Errorf("mq subscribe: %s", err)
	}

	go func() {
		<-n.ctx.Done()

		err := sub.Unsubscribe()
		if err != nil {
			n.lg.Error("nats unsubscribe failure", zap.String("subject", sub.Subject), zap.Error(err))
			return
		}

		n.lg.Info("unsubscribed", zap.String("subject", sub.Subject))
	}()

	go func() {
		for msg := range ch {
			err := hf(msg.Data)
			if err != nil {
				n.lg.Error("failed to handle message", zap.String("topic", topic), zap.Error(err))
			}
		}
	}()

	return nil
}

// Publish publishes a message into a topic
func (n *Nats) Publish(topic string, msg []byte) error {
	err := n.Conn.Publish(topic, msg)
	if err != nil {
		return fmt.Errorf("publish message: %s", err)
	}

	return nil
}
