package wechataccount

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	messages "orderapp/internal/application/messagecenter"
	app "orderapp/internal/application/officialaccount"
	"regexp"
	"strconv"
	"strings"
)

type TemplateValidationError struct{ Reason string }

func (e *TemplateValidationError) Error() string { return e.Reason }

type APIError struct{ Code int }

func (e *APIError) Error() string { return fmt.Sprintf("微信接口错误 %d", e.Code) }

type TemplateMapping struct {
	ID            string              `json:"id"`
	Fields        map[string]string   `json:"fields"`
	ApprovedEnums map[string][]string `json:"approved_enums,omitempty"`
}

var templateField = regexp.MustCompile(`^(thing|character_string|time|date|number|amount|phrase|name|phone_number|car_number|const|letter|symbol)[0-9]+$`)

func (t TemplateMapping) Validate() error {
	if strings.TrimSpace(t.ID) == "" || len(t.ID) > 128 || len(t.Fields) == 0 || len(t.Fields) > 20 {
		return errors.New("请配置业务模板及字段映射")
	}
	allowed := messages.OrderNotice{}.Values()
	// Permit administrators to prepare these mappings while finance delivery is
	// disabled. No finance payload is supplied to the order sender.
	for _, source := range []string{"receipt_date", "receivable_amount", "receipt_no", "received_amount", "bill_no", "bill_amount", "bill_period"} {
		allowed[source] = ""
	}
	for field, source := range t.Fields {
		if !templateField.MatchString(field) && field != "first" && field != "remark" && !regexp.MustCompile(`^keyword[0-9]+$`).MatchString(field) {
			return errors.New("模板字段无效；一次性订阅模板不能用于持续订单通知")
		}
		if strings.HasPrefix(field, "const") {
			if len(t.ApprovedEnums[field]) == 0 {
				return errors.New("常量字段尚未配置审核通过的枚举值")
			}
			for _, value := range t.ApprovedEnums[field] {
				if strings.TrimSpace(value) == "" || len([]rune(value)) > 20 {
					return errors.New("常量枚举值无效")
				}
			}
		}
		if _, ok := allowed[source]; !ok {
			return errors.New("不支持的订单模板字段")
		}
	}
	return nil
}

// Conservative local validation; a valid value can still be rejected by WeChat.
func validateTemplateValue(field, value string) error {
	if strings.HasPrefix(field, "character_string") {
		if !regexp.MustCompile(`^[!-~]{1,32}$`).MatchString(value) {
			return &TemplateValidationError{Reason: "编号或单号格式无效"}
		}
	}
	if strings.HasPrefix(field, "amount") {
		if !regexp.MustCompile(`^(?:[¥￥])?[0-9]+(?:\.[0-9]+)?(?:元)?$`).MatchString(value) {
			return &TemplateValidationError{Reason: "金额格式无效"}
		}
		digits := 0
		for _, r := range value {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits > 12 {
			return &TemplateValidationError{Reason: "金额超过字段长度"}
		}
	}
	if strings.HasPrefix(field, "time") || strings.HasPrefix(field, "date") {
		part := `(?:[0-9]{4}[-./][0-9]{1,2}[-./][0-9]{1,2}|[0-9]{4}年[0-9]{1,2}月[0-9]{1,2}日|[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)(?: [0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)?`
		if !regexp.MustCompile(`^` + part + `(?:~` + part + `)?$`).MatchString(value) {
			return &TemplateValidationError{Reason: "时间格式无效"}
		}
	}
	return nil
}

func (w *WechatClient) SendOrderTemplate(ctx context.Context, openid string, t TemplateMapping, n messages.OrderNotice, employee bool) (string, error) {
	if err := t.Validate(); err != nil {
		return "", &APIError{Code: 40037}
	}
	if strings.TrimSpace(openid) == "" {
		return "", &APIError{Code: 40003}
	}
	data := map[string]map[string]string{}
	values := n.Values()
	for field, source := range t.Fields {
		value := values[source]
		if strings.HasPrefix(field, "const") {
			approved := false
			for _, v := range t.ApprovedEnums[field] {
				if value == v {
					approved = true
					break
				}
			}
			if !approved {
				return "", &TemplateValidationError{Reason: "常量字段的值尚未通过审核或未配置"}
			}
		}
		if n.Test && ((source == "status" && !strings.HasPrefix(field, "const")) || (source == "items" && strings.HasPrefix(field, "thing"))) {
			value = "测试·" + value
		}
		limit := 0
		if strings.HasPrefix(field, "thing") {
			limit = 20
		}
		if strings.HasPrefix(field, "phrase") {
			limit = 5
		}
		if limit > 0 && len([]rune(value)) > limit {
			value = string([]rune(value)[:limit])
		}
		if err := validateTemplateValue(field, value); err != nil {
			return "", err
		}
		data[field] = map[string]string{"value": value}
	}
	token, err := w.accessToken(ctx)
	if err != nil {
		return "", &TokenError{}
	}
	body := map[string]any{"touser": openid, "template_id": t.ID, "data": data}
	if w.Config.MiniAppID != "" {
		body["miniprogram"] = map[string]string{"appid": w.Config.MiniAppID, "pagepath": n.MiniPath(employee)}
	}
	var out struct {
		ID json.Number `json:"msgid"`
	}
	err = w.call(ctx, "/cgi-bin/message/template/send", url.Values{"access_token": {token}}, body, &out)
	var api *APIError
	if errors.As(err, &api) && (api.Code == 40001 || api.Code == 40014 || api.Code == 42001) {
		w.mu.Lock()
		w.token = ""
		w.expires = w.expires.AddDate(-1, 0, 0)
		w.mu.Unlock()
	}
	if err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("微信返回缺少消息编号，结果待确认")
	}
	return out.ID.String(), nil
}

type TokenError struct{}

func (*TokenError) Error() string { return "获取微信凭证失败，尚未发送" }
func TemplateFailureState(err error) string {
	if err == nil {
		return "accepted"
	}
	var validation *TemplateValidationError
	if errors.As(err, &validation) {
		return "failed"
	}
	var token *TokenError
	if errors.As(err, &token) {
		return "retry"
	}
	var api *APIError
	if !errors.As(err, &api) {
		return "unknown"
	}
	switch api.Code {
	case 43004:
		return "skipped"
	case -1, 45009, 45011, 40001, 40014, 42001:
		return "retry"
	default:
		return "failed"
	}
}

type Template struct {
	ID      string `json:"template_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (w *WechatClient) Templates(ctx context.Context) ([]Template, error) {
	token, err := w.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var out struct {
		Rows []Template `json:"template_list"`
	}
	err = w.call(ctx, "/cgi-bin/template/get_all_private_template", url.Values{"access_token": {token}}, nil, &out)
	return out.Rows, err
}
func (w *WechatClient) Subscribed(ctx context.Context, openid string) (bool, error) {
	token, err := w.accessToken(ctx)
	if err != nil {
		return false, err
	}
	var out struct {
		Subscribe int `json:"subscribe"`
	}
	err = w.call(ctx, "/cgi-bin/user/info", url.Values{"access_token": {token}, "openid": {openid}}, nil, &out)
	return out.Subscribe == 1, err
}
func ProviderID(id int64) string { return strconv.FormatInt(id, 10) }

func (w *WechatClient) NotificationQRCode(ctx context.Context, challenge string) (string, error) {
	scene, err := app.NotificationScene(challenge)
	if err != nil {
		return "", err
	}
	token, err := w.accessToken(ctx)
	if err != nil {
		return "", err
	}
	var out struct {
		Ticket string `json:"ticket"`
	}
	err = w.call(ctx, "/cgi-bin/qrcode/create", url.Values{"access_token": {token}}, map[string]any{"expire_seconds": 300, "action_name": "QR_STR_SCENE", "action_info": map[string]any{"scene": map[string]string{"scene_str": scene}}}, &out)
	if err != nil {
		return "", err
	}
	if out.Ticket == "" {
		return "", errors.New("微信未返回关注二维码")
	}
	return "https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket=" + url.QueryEscape(out.Ticket), nil
}
