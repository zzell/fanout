package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zzell/fanout/internal/cache"
	"github.com/zzell/fanout/internal/cache/mock"
	mock2 "github.com/zzell/fanout/internal/mq/mock"
	"go.uber.org/zap"
)

func TestService_StatusChangeHandler(t *testing.T) {
	var (
		ctx       = context.Background()
		ctrl      = gomock.NewController(t)
		cacheMock = mock.NewMockCache(ctrl)
		mqMock    = mock2.NewMockMQ(ctrl)
		lg        = zap.NewNop()
		vdt       = validator.New()
		cfg       = &Config{
			StatusChangeSubj: "sc",
			SendDigestSubj:   "sd",
			AlarmDigestSubj:  "ad",
		}
	)

	mqMock.EXPECT().Subscribe(cfg.StatusChangeSubj, gomock.Any()).Return(nil)
	mqMock.EXPECT().Subscribe(cfg.SendDigestSubj, gomock.Any()).Return(nil)

	srv, e := New(ctx, lg, cfg, cacheMock, mqMock, vdt)
	require.NoError(t, e)

	t.Run("positive_first_status_change", func(t *testing.T) {
		var (
			alarmID = uuid.New().String()
			userID  = uuid.New().String()
		)

		msg := StatusChangeMsg{
			AlarmID:   alarmID,
			UserID:    userID,
			Status:    AlarmStatusWarning.String(),
			ChangedAt: time.Now().String(),
		}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		cacheMock.EXPECT().Get(userID).Return(nil, cache.ErrNotFound)
		cacheMock.EXPECT().Set(userID, gomock.Any()).Return(nil)

		err = srv.StatusChangeHandler(b)
		require.NoError(t, err)
	})

	t.Run("positive_update_status", func(t *testing.T) {
		var (
			alarmID = uuid.New().String()
			userID  = uuid.New().String()
		)

		msg := StatusChangeMsg{
			AlarmID:   alarmID,
			UserID:    userID,
			Status:    AlarmStatusWarning.String(),
			ChangedAt: time.Now().String(),
		}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		digest := AlarmDigestMsg{
			UserID: userID,
			ActiveAlarms: []ActiveAlarm{
				{
					AlarmID:         alarmID,
					Status:          AlarmStatusCleared.String(),
					LatestChangedAt: time.Now().String(),
				},
			},
		}

		bdigest, err := json.Marshal(digest)
		require.NoError(t, err)

		cacheMock.EXPECT().Get(userID).Return(bdigest, nil)
		cacheMock.EXPECT().Set(userID, gomock.Any()).Return(nil)

		err = srv.StatusChangeHandler(b)
		require.NoError(t, err)
	})

	t.Run("negative_illegal_msg_format", func(t *testing.T) {
		e = srv.StatusChangeHandler([]byte("{not a json"))
		require.Error(t, e)
	})

	t.Run("negative_invalid_message", func(t *testing.T) {
		msg := StatusChangeMsg{
			AlarmID:   "",
			UserID:    "",
			Status:    AlarmStatusWarning.String(),
			ChangedAt: time.Now().String(),
		}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		err = srv.StatusChangeHandler(b)
		require.Error(t, err)
	})

	t.Run("negative_cache_error", func(t *testing.T) {
		var (
			alarmID = uuid.New().String()
			userID  = uuid.New().String()
		)

		msg := StatusChangeMsg{
			AlarmID:   alarmID,
			UserID:    userID,
			Status:    AlarmStatusWarning.String(),
			ChangedAt: time.Now().String(),
		}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		cacheMock.EXPECT().Get(userID).Return(nil, errors.New(""))

		err = srv.StatusChangeHandler(b)
		require.Error(t, err)
	})
}

func TestService_SendDigestHandler(t *testing.T) {
	var (
		ctx       = context.Background()
		ctrl      = gomock.NewController(t)
		cacheMock = mock.NewMockCache(ctrl)
		mqMock    = mock2.NewMockMQ(ctrl)
		lg        = zap.NewNop()
		vdt       = validator.New()
		cfg       = &Config{AlarmDigestSubj: "ad"}
		srv       = &Service{
			ctx:       ctx,
			L:         lg,
			Cfg:       cfg,
			Cache:     cacheMock,
			Validator: vdt,
			MQ:        mqMock,
		}
	)

	t.Run("positive", func(t *testing.T) {
		var userID = uuid.New().String()
		var msg = SendDigestMsg{UserID: userID}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		// we don't care about stored value
		cacheMock.EXPECT().Get(userID).Return(nil, nil)
		mqMock.EXPECT().Publish(cfg.AlarmDigestSubj, nil).Return(nil)

		err = srv.SendDigestHandler(b)
		require.NoError(t, err)
	})

	t.Run("negative_illegal_message", func(t *testing.T) {
		err := srv.SendDigestHandler([]byte("nop"))
		require.Error(t, err)
	})

	t.Run("negative_invalid_message", func(t *testing.T) {
		var msg = SendDigestMsg{UserID: ""}
		b, err := json.Marshal(msg)
		require.NoError(t, err)

		err = srv.SendDigestHandler(b)
		require.Error(t, err)
	})

	t.Run("negative_cache_error", func(t *testing.T) {
		var userID = uuid.New().String()
		var msg = SendDigestMsg{UserID: userID}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		cacheMock.EXPECT().Get(userID).Return(nil, errors.New(""))

		err = srv.SendDigestHandler(b)
		require.Error(t, err)
	})

	t.Run("negative_mq_error", func(t *testing.T) {
		var userID = uuid.New().String()
		var msg = SendDigestMsg{UserID: userID}

		b, err := json.Marshal(msg)
		require.NoError(t, err)

		cacheMock.EXPECT().Get(userID).Return(nil, nil)
		mqMock.EXPECT().Publish(cfg.AlarmDigestSubj, nil).Return(errors.New(""))

		err = srv.SendDigestHandler(b)
		require.Error(t, err)
	})
}
