package messagecenter

import (
	"context"
	"strings"
)

const ChannelERPPlatform = "erp_platform"

type DeliveryCommand struct {
	Channel          string
	TargetType       string
	TargetKey        string
	TargetEmployeeID int64
}

type PublishCommand struct {
	EventKey   string
	Topic      string
	EventType  string
	SourceType string
	SourceID   int64
	Actor      string
	Title      string
	Body       string
	Tone       string
	Payload    map[string]any
	Deliveries []DeliveryCommand
}

type NotificationQuery struct {
	EmployeeID int64
	Channel    string
	Status     string
	AfterID    int64
	Limit      int
}

type Notification struct {
	ID         int64          `json:"id"`
	Topic      string         `json:"topic"`
	EventType  string         `json:"event_type"`
	SourceType string         `json:"source_type"`
	SourceID   int64          `json:"source_id"`
	Title      string         `json:"title"`
	Body       string         `json:"body"`
	Tone       string         `json:"tone"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  string         `json:"created_at"`
	Read       bool           `json:"read"`
}

type Repository interface {
	Publish(context.Context, PublishCommand) (int64, error)
	ListNotifications(context.Context, NotificationQuery) ([]Notification, error)
	MarkRead(context.Context, int64, int64) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Publish(ctx context.Context, cmd PublishCommand) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	cmd.EventKey = strings.TrimSpace(cmd.EventKey)
	cmd.Topic = strings.TrimSpace(cmd.Topic)
	cmd.EventType = strings.TrimSpace(cmd.EventType)
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Title = strings.TrimSpace(cmd.Title)
	cmd.Body = strings.TrimSpace(cmd.Body)
	cmd.Tone = normalizeTone(cmd.Tone)
	if cmd.Payload == nil {
		cmd.Payload = map[string]any{}
	}
	if len(cmd.Deliveries) == 0 {
		cmd.Deliveries = []DeliveryCommand{{
			Channel:    ChannelERPPlatform,
			TargetType: "permission",
			TargetKey:  "orders.read",
		}}
	}
	for i := range cmd.Deliveries {
		cmd.Deliveries[i].Channel = normalizeChannel(cmd.Deliveries[i].Channel)
		cmd.Deliveries[i].TargetType = normalizeTargetType(cmd.Deliveries[i].TargetType)
		cmd.Deliveries[i].TargetKey = strings.TrimSpace(cmd.Deliveries[i].TargetKey)
	}
	return s.repo.Publish(ctx, cmd)
}

func (s *Service) ListNotifications(ctx context.Context, query NotificationQuery) ([]Notification, error) {
	if s == nil || s.repo == nil {
		return []Notification{}, nil
	}
	query.Channel = normalizeChannel(query.Channel)
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	return s.repo.ListNotifications(ctx, query)
}

func (s *Service) MarkRead(ctx context.Context, eventID, employeeID int64) error {
	if s == nil || s.repo == nil || eventID <= 0 || employeeID <= 0 {
		return nil
	}
	return s.repo.MarkRead(ctx, eventID, employeeID)
}

func normalizeChannel(channel string) string {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return ChannelERPPlatform
	}
	return channel
}

func normalizeTargetType(targetType string) string {
	targetType = strings.TrimSpace(targetType)
	switch targetType {
	case "employee", "permission", "broadcast", "mini_user", "phone":
		return targetType
	default:
		return "broadcast"
	}
}

func normalizeTone(tone string) string {
	tone = strings.TrimSpace(tone)
	switch tone {
	case "success", "warning", "info", "danger":
		return tone
	default:
		return "info"
	}
}
