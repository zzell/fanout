package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/zzell/fanout/config"
	"github.com/zzell/fanout/internal/cache"
	"github.com/zzell/fanout/internal/mq"
	"github.com/zzell/fanout/internal/service"
	"go.uber.org/zap"
)

//go:embed config.yaml
var configfile []byte // nolint:gochecknoglobals

func main() {
	lg, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalln("failed to init logger: ", err)
	}

	var (
		ctx, cancel = context.WithCancel(context.Background())
		sig         = make(chan os.Signal)
	)

	signal.Notify(sig, os.Interrupt)

	go func() {
		interrupt := <-sig
		lg.Info("received shutdown signal", zap.String("signal", interrupt.String()))
		cancel()
	}()

	cfg, err := config.NewConfig(configfile)
	if err != nil {
		lg.Fatal("failed to read configuration file", zap.Error(err))
	}

	natsmq, err := mq.NewNats(ctx, cfg.MqConfig, lg)
	if err != nil {
		lg.Fatal("failed to create nats client", zap.Error(err))
	}

	redis, err := cache.NewRedis(ctx, cfg.CacheConfig, lg)
	if err != nil {
		lg.Fatal("failed to create redis client", zap.Error(err))
	}

	v := validator.New()

	_, err = service.New(ctx, lg, cfg.ServiceConfig, redis, natsmq, v)
	if err != nil {
		lg.Fatal("failed to create service", zap.Error(err))
	}

	lg.Info("service up and running")

	<-ctx.Done()
	lg.Info("gracefully shutting down...")
	time.Sleep(time.Second * 1) // wait a bit until everything is done
}
