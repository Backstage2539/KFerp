package messagecenter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const ChannelERPPlatform = "erp_platform"
const ChannelExternalIM = "external_im"

type DeliveryCommand struct {
	Channel          string
	TargetType       string
	TargetKey        string
	TargetEmployeeID int64
	TemplateKey      string
	AdapterKey       string
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

type Rule struct {
	ID               int64          `json:"id"`
	Code             string         `json:"code"`
	Name             string         `json:"name"`
	Enabled          bool           `json:"enabled"`
	Topic            string         `json:"topic"`
	EventType        string         `json:"event_type"`
	SourceType       string         `json:"source_type"`
	Channel          string         `json:"channel"`
	TargetType       string         `json:"target_type"`
	TargetKey        string         `json:"target_key"`
	TargetEmployeeID int64          `json:"target_employee_id"`
	TemplateKey      string         `json:"template_key"`
	AdapterKey       string         `json:"adapter_key"`
	Tone             string         `json:"tone"`
	PayloadMatch     map[string]any `json:"payload_match"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

type RuleQuery struct {
	Topic      string
	EventType  string
	SourceType string
}

type SaveRuleCommand struct {
	ID               int64          `json:"id"`
	Code             string         `json:"code"`
	Name             string         `json:"name"`
	Enabled          *bool          `json:"enabled"`
	Topic            string         `json:"topic"`
	EventType        string         `json:"event_type"`
	SourceType       string         `json:"source_type"`
	Channel          string         `json:"channel"`
	TargetType       string         `json:"target_type"`
	TargetKey        string         `json:"target_key"`
	TargetEmployeeID int64          `json:"target_employee_id"`
	TemplateKey      string         `json:"template_key"`
	AdapterKey       string         `json:"adapter_key"`
	Tone             string         `json:"tone"`
	PayloadMatch     map[string]any `json:"payload_match"`
}

type ExternalDelivery struct {
	DeliveryID       int64          `json:"delivery_id"`
	EventID          int64          `json:"event_id"`
	EventType        string         `json:"event_type"`
	SourceType       string         `json:"source_type"`
	SourceID         int64          `json:"source_id"`
	Channel          string         `json:"channel"`
	AdapterKey       string         `json:"adapter_key"`
	TargetType       string         `json:"target_type"`
	TargetKey        string         `json:"target_key"`
	TargetEmployeeID int64          `json:"target_employee_id"`
	TemplateKey      string         `json:"template_key"`
	Title            string         `json:"title"`
	Body             string         `json:"body"`
	Payload          map[string]any `json:"payload"`
}

type ExternalDeliveryResult struct {
	ProviderMessageID string         `json:"provider_message_id,omitempty"`
	Response          map[string]any `json:"response,omitempty"`
}

type ChannelAdapter interface {
	Channel() string
	Send(context.Context, ExternalDelivery) (ExternalDeliveryResult, error)
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
	ListRules(context.Context) ([]Rule, error)
	ListActiveRules(context.Context, RuleQuery) ([]Rule, error)
	SaveRule(context.Context, SaveRuleCommand) (Rule, error)
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
	ruleDeliveries, err := s.ruleDeliveries(ctx, cmd)
	if err != nil {
		return 0, err
	}
	cmd.Deliveries = append(ruleDeliveries, cmd.Deliveries...)
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
		cmd.Deliveries[i].TemplateKey = strings.TrimSpace(cmd.Deliveries[i].TemplateKey)
		cmd.Deliveries[i].AdapterKey = strings.TrimSpace(cmd.Deliveries[i].AdapterKey)
	}
	cmd.Deliveries = dedupeDeliveries(cmd.Deliveries)
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

func (s *Service) ListRules(ctx context.Context) ([]Rule, error) {
	if s == nil || s.repo == nil {
		return []Rule{}, nil
	}
	return s.repo.ListRules(ctx)
}

func (s *Service) SaveRule(ctx context.Context, cmd SaveRuleCommand) (Rule, error) {
	if s == nil || s.repo == nil {
		return Rule{}, nil
	}
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Topic = strings.TrimSpace(cmd.Topic)
	cmd.EventType = strings.TrimSpace(cmd.EventType)
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	cmd.Channel = normalizeChannel(cmd.Channel)
	cmd.TargetType = normalizeTargetType(cmd.TargetType)
	cmd.TargetKey = strings.TrimSpace(cmd.TargetKey)
	cmd.TemplateKey = strings.TrimSpace(cmd.TemplateKey)
	cmd.AdapterKey = strings.TrimSpace(cmd.AdapterKey)
	cmd.Tone = normalizeTone(cmd.Tone)
	if cmd.Code == "" {
		return Rule{}, fmt.Errorf("code required")
	}
	if cmd.EventType == "" {
		return Rule{}, fmt.Errorf("event_type required")
	}
	if cmd.TargetType == "" {
		return Rule{}, fmt.Errorf("target_type required")
	}
	if cmd.TargetType == "employee" && cmd.TargetEmployeeID <= 0 {
		if id, err := strconv.ParseInt(cmd.TargetKey, 10, 64); err == nil {
			cmd.TargetEmployeeID = id
		}
	}
	if cmd.TargetType == "employee" && cmd.TargetEmployeeID <= 0 {
		return Rule{}, fmt.Errorf("target_employee_id required")
	}
	if cmd.TargetType != "broadcast" && cmd.TargetType != "employee" && cmd.TargetKey == "" {
		return Rule{}, fmt.Errorf("target_key required")
	}
	if cmd.Enabled == nil {
		enabled := true
		cmd.Enabled = &enabled
	}
	return s.repo.SaveRule(ctx, cmd)
}

func (s *Service) ruleDeliveries(ctx context.Context, cmd PublishCommand) ([]DeliveryCommand, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(cmd.EventType) == "" {
		return nil, nil
	}
	rules, err := s.repo.ListActiveRules(ctx, RuleQuery{
		Topic:      cmd.Topic,
		EventType:  cmd.EventType,
		SourceType: cmd.SourceType,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DeliveryCommand, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled || !payloadMatches(rule.PayloadMatch, cmd.Payload) {
			continue
		}
		delivery := DeliveryCommand{
			Channel:          rule.Channel,
			TargetType:       rule.TargetType,
			TargetKey:        rule.TargetKey,
			TargetEmployeeID: rule.TargetEmployeeID,
			TemplateKey:      rule.TemplateKey,
			AdapterKey:       rule.AdapterKey,
		}
		if delivery.TargetType == "employee" && delivery.TargetEmployeeID <= 0 {
			if id, err := strconv.ParseInt(strings.TrimSpace(delivery.TargetKey), 10, 64); err == nil {
				delivery.TargetEmployeeID = id
			}
		}
		out = append(out, delivery)
	}
	return out, nil
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
	case "employee", "permission", "role", "broadcast", "mini_user", "phone", "order_customer", "order_responsible", "channel_identity":
		return targetType
	default:
		return "broadcast"
	}
}

func payloadMatches(match, payload map[string]any) bool {
	if len(match) == 0 {
		return true
	}
	for key, want := range match {
		got, ok := payload[key]
		if !ok {
			return false
		}
		if !jsonValuesEqual(got, want) {
			return false
		}
	}
	return true
}

func jsonValuesEqual(got, want any) bool {
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	return string(gb) == string(wb)
}

func dedupeDeliveries(rows []DeliveryCommand) []DeliveryCommand {
	seen := map[string]bool{}
	out := make([]DeliveryCommand, 0, len(rows))
	for _, row := range rows {
		key := strings.Join([]string{
			row.Channel,
			row.TargetType,
			row.TargetKey,
			strconv.FormatInt(row.TargetEmployeeID, 10),
			row.TemplateKey,
			row.AdapterKey,
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, row)
	}
	return out
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
