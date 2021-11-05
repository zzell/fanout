//+build integration

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zzell/fanout/internal/mq"
	"github.com/zzell/fanout/internal/service"
	"go.uber.org/zap"
)

const (
	natsServer = "nats://localhost:4222"

	statusChangeTopic = "AlarmStatusChanged"
	sendDigestTopic   = "SendAlarmDigest"
	alarmDigestTopic  = "AlarmDigest"
)

func TestMain(m *testing.M) {
	go main()
	time.Sleep(time.Second)

	m.Run()
}

func TestIntegration_Service(t *testing.T) {
	var (
		ctx = context.Background()
		lg  = zap.NewNop()
	)

	n, err := mq.NewNats(ctx, &mq.Config{ServerURL: natsServer}, lg)
	require.NoError(t, err)

	t.Run("positive", func(t *testing.T) {
		userID := uuid.New().String()
		alarmID := uuid.New().String()
		done := make(chan struct{})

		var f = func(msg []byte) error {
			var m = new(service.AlarmDigestMsg)
			err := json.Unmarshal(msg, m)
			require.NoError(t, err)

			require.Equal(t, userID, m.UserID)
			require.Len(t, m.ActiveAlarms, 1)
			require.Equal(t, m.ActiveAlarms[0].Status, service.AlarmStatusWarning.String())

			done <- struct{}{}
			return nil
		}

		err = n.Subscribe(alarmDigestTopic, f)
		require.NoError(t, err)

		m1 := service.StatusChangeMsg{
			AlarmID:   alarmID,
			UserID:    userID,
			Status:    service.AlarmStatusCleared.String(),
			ChangedAt: time.Now().String(),
		}

		b, err := json.Marshal(m1)
		require.NoError(t, err)
		err = n.Publish(statusChangeTopic, b)
		require.NoError(t, err)

		m2 := service.StatusChangeMsg{
			AlarmID:   alarmID,
			UserID:    userID,
			Status:    service.AlarmStatusWarning.String(),
			ChangedAt: time.Now().String(),
		}

		b, err = json.Marshal(m2)
		require.NoError(t, err)
		err = n.Publish(statusChangeTopic, b)
		require.NoError(t, err)

		time.Sleep(time.Second)

		m3 := service.SendDigestMsg{UserID: userID}
		b, err = json.Marshal(m3)
		require.NoError(t, err)
		err = n.Publish(sendDigestTopic, b)

		<-done
	})
}
