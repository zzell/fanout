package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/zzell/fanout/internal/cache"
	"github.com/zzell/fanout/internal/mq"
	"go.uber.org/zap"
)

const alarmStatusValidationTag = "alarm_status"

// status enums
const (
	AlarmStatusCleared  alarmStatus = iota // the alarm is not triggered
	AlarmStatusWarning                     // first threshold is reached
	AlarmStatusCritical                    // second threshold is reached
)

var alarmStatusText = [...]string{
	AlarmStatusCleared:  "CREATED",
	AlarmStatusWarning:  "WARNING",
	AlarmStatusCritical: "CRITICAL",
}

type alarmStatus int

func (s alarmStatus) String() string { return alarmStatusText[s] }

func alarmStatusValidationFunc(level validator.FieldLevel) bool {
	val := level.Field().String()

	for _, st := range alarmStatusText {
		if val == st {
			return true
		}
	}

	return false
}

// Config service configuration
type Config struct {
	StatusChangeSubj string `yaml:"status_change_subj"`
	SendDigestSubj   string `yaml:"send_digest_subj"`
	AlarmDigestSubj  string `yaml:"alarm_digest_subj"`
}

// Service listener
type Service struct {
	ctx context.Context

	L         *zap.Logger
	Cfg       *Config
	Cache     cache.Cache
	Validator *validator.Validate
	MQ        mq.MQ
}

// New Service constructor
func New(ctx context.Context, lg *zap.Logger, cfg *Config, ch cache.Cache, m mq.MQ, v *validator.Validate) (*Service, error) {
	err := v.RegisterValidation(alarmStatusValidationTag, alarmStatusValidationFunc)
	if err != nil {
		return nil, fmt.Errorf("register alarm status validation: %w", err)
	}

	srv := &Service{
		ctx:       ctx,
		L:         lg,
		Cfg:       cfg,
		Cache:     ch,
		Validator: v,
		MQ:        m,
	}

	err = m.Subscribe(cfg.StatusChangeSubj, srv.StatusChangeHandler)
	if err != nil {
		return nil, fmt.Errorf("subscribe to mq, topic: %s: %w", cfg.StatusChangeSubj, err)
	}

	err = m.Subscribe(cfg.SendDigestSubj, srv.SendDigestHandler)
	if err != nil {
		return nil, fmt.Errorf("subscribe to mq, topic: %s: %w", cfg.SendDigestSubj, err)
	}

	return srv, nil
}

func (a *AlarmDigestMsg) upsert(alarm ActiveAlarm) {
	for i, aa := range a.ActiveAlarms {
		if aa.AlarmID == alarm.AlarmID {
			a.ActiveAlarms[i].Status = alarm.Status
			a.ActiveAlarms[i].LatestChangedAt = alarm.LatestChangedAt
			return
		}
	}

	a.ActiveAlarms = append(a.ActiveAlarms, alarm)
}

// StatusChangeHandler handles status change events
func (s *Service) StatusChangeHandler(msg []byte) error {
	var body = new(StatusChangeMsg)

	err := json.Unmarshal(msg, body)
	if err != nil {
		return fmt.Errorf("unmarshal message body: %w", err)
	}

	err = s.Validator.StructCtx(s.ctx, body)
	if err != nil {
		if errors.Is(err, &validator.InvalidValidationError{}) {
			return fmt.Errorf("validation failure: %w", err)
		}
		return fmt.Errorf("invalid message body: %w", err)
	}

	s.L.Info("status change event", zap.String("user_id", body.UserID), zap.String("status", body.Status))

	err = s.updateAlarmStatus(body)
	if err != nil {
		return fmt.Errorf("append alarm digest into cache: %w", err)
	}

	return nil
}

// SendDigestHandler handles send digest events
func (s *Service) SendDigestHandler(msg []byte) error {
	var body = new(SendDigestMsg)

	err := json.Unmarshal(msg, body)
	if err != nil {
		return fmt.Errorf("unmarshal message body: %w", err)
	}

	err = s.Validator.Struct(body)
	if errors.Is(err, &validator.InvalidValidationError{}) {
		return fmt.Errorf("validation failure: %w", err)
	}

	if err != nil {
		return fmt.Errorf("invalid message body: %w", err)
	}

	s.L.Info("send digest event", zap.String("user_id", body.UserID))

	b, err := s.Cache.Get(body.UserID)
	if err != nil {
		return fmt.Errorf("cache Get, key: %s: %w", body.UserID, err)
	}

	err = s.MQ.Publish(s.Cfg.AlarmDigestSubj, b)
	if err != nil {
		return fmt.Errorf("publish message, topic: %s: %w", s.Cfg.AlarmDigestSubj, err)
	}

	return nil
}

func (s *Service) updateAlarmStatus(body *StatusChangeMsg) error {
	digest, err := s.Cache.Get(body.UserID)
	if errors.Is(err, cache.ErrNotFound) {
		b, e := json.Marshal(AlarmDigestMsg{
			UserID: body.UserID,
			ActiveAlarms: []ActiveAlarm{
				{
					AlarmID:         body.AlarmID,
					Status:          body.Status,
					LatestChangedAt: body.ChangedAt,
				},
			},
		})

		if e != nil {
			return fmt.Errorf("marshal alarm digest body: %w", e)
		}

		e = s.Cache.Set(body.UserID, b)
		if e != nil {
			return fmt.Errorf("cache Set: %w", e)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("cache Get: %w", err)
	}

	var cbody = new(AlarmDigestMsg)

	err = json.Unmarshal(digest, cbody)
	if err != nil {
		return fmt.Errorf("unmarshal digest body from cache: %w", err)
	}

	cbody.upsert(ActiveAlarm{
		AlarmID:         body.AlarmID,
		Status:          body.Status,
		LatestChangedAt: body.ChangedAt,
	})

	b, err := json.Marshal(cbody)
	if err != nil {
		return fmt.Errorf("marshal alarm digest body: %w", err)
	}

	err = s.Cache.Set(body.UserID, b)
	if err != nil {
		return fmt.Errorf("cache Set: %w", err)
	}

	return nil
}
