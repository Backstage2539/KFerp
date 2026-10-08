package officialaccount

import (
	"errors"
	"fmt"
	"net/url"
	portal "orderapp/internal/application/customerportal"
	"strings"
	"unicode/utf8"
)

var ErrDenied = errors.New("请先登录小程序并完成客户认证或公众号绑定")
var ErrInvalidCode = errors.New("绑定码无效或已过期，请在小程序重新获取")
var ErrBindingConflict = errors.New("此公众号已绑定其他账号，请先解绑")
var ErrRateLimited = errors.New("尝试次数过多，请 5 分钟后重试")
var ErrUnavailable = errors.New("入口已停用或所选价格表版本不可用")

type Entry struct {
	Key           string `json:"key"`
	Scope         string `json:"-"`
	Name          string `json:"name"`
	PublicationID int64  `json:"publication_id"`
	TypeKey       string `json:"type_key,omitempty"`
	TypeName      string `json:"type_name,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
	Visibility    string `json:"visibility"`
	Enabled       bool   `json:"enabled"`
	Revision      int64  `json:"revision"`
	OwnerType     string `json:"owner_type"`
	OwnerKey      string `json:"owner_key"`
	Version       string `json:"version"`
	TableName     string `json:"table_name"`
	Status        string `json:"status"`
	PagePath      string `json:"page_path"`
}
type Version struct {
	ID        int64  `json:"id"`
	Version   string `json:"version"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	TableKey  string `json:"table_key,omitempty"`
	OwnerType string `json:"owner_type,omitempty"`
	OwnerKey  string `json:"owner_key,omitempty"`
}

func CheckEntry(e Entry, c *portal.CurrentContext) error {
	if !e.Enabled || e.Status != "published" {
		return ErrUnavailable
	}
	if e.Visibility == "public" {
		if e.OwnerType != "official" {
			return ErrDenied
		}
		return nil
	}
	if c == nil || c.CurrentCustomerID <= 0 || c.AccountType == "employee" {
		return ErrDenied
	}
	if !c.HasCapability(portal.CapabilityBeanList) || !c.HasCapability(portal.CapabilityProductOrder) {
		return ErrDenied
	}
	if e.OwnerType == "customer" && e.OwnerKey != fmt.Sprint(c.CurrentCustomerID) {
		return ErrDenied
	}
	if e.OwnerType != "official" && e.OwnerType != "customer" {
		return ErrDenied
	}
	return nil
}
func Clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
func OrderSummary(customer string, rows []portal.CustomerOrderSummary) string {
	if len(rows) == 0 {
		return Clip(customer, 90) + "\n暂无订单"
	}
	if len(rows) > 3 {
		rows = rows[:3]
	}
	out := Clip(customer, 90) + fmt.Sprintf("｜最近 %d 笔订单\n", len(rows))
	for _, r := range rows {
		out += "\n" + Clip(r.OrderNo, 60) + " · " + Clip(r.OrderDate, 30) + "\n"
		out += "金额 ¥" + Clip(r.GrandTotal, 24) + "｜" + Clip(r.PayStatus, 30) + "\n"
		out += "生产：" + Clip(r.ProcessStatus, 30) + "｜发货：" + Clip(r.ShipStatus, 30) + "\n"
		if r.ShipTrackingNo != "" {
			out += "运单：" + Clip(r.ShipTrackingNo, 64) + "\n"
		}
		for i, item := range r.Items {
			if i >= 2 {
				out += "其余商品请查看全部订单\n"
				break
			}
			out += Clip(item.ItemName, 60) + " " + Clip(item.Spec, 36) + " × " + Clip(item.Qty, 15) + Clip(item.Unit, 12) + "\n"
		}
	}
	return Clip(out, 1800) + "\n完整明细请点击「我的订单 → 全部订单」。"
}

type Button struct {
	Name       string   `json:"name"`
	Type       string   `json:"type,omitempty"`
	Key        string   `json:"key,omitempty"`
	AppID      string   `json:"appid,omitempty"`
	PagePath   string   `json:"pagepath,omitempty"`
	URL        string   `json:"url,omitempty"`
	MediaID    string   `json:"media_id,omitempty"`
	SubButtons []Button `json:"sub_button,omitempty"`
}
type Menu struct {
	Buttons []Button `json:"button"`
}

func ValidateMenu(m Menu, miniAppID string) error {
	if len(m.Buttons) < 1 || len(m.Buttons) > 3 {
		return errors.New("菜单需要 1～3 个一级菜单")
	}
	for _, b := range m.Buttons {
		if len(b.Name) == 0 || len([]rune(b.Name)) > 4 {
			return errors.New("一级菜单名称最多 4 个汉字")
		}
		if len(b.SubButtons) > 0 {
			if len(b.SubButtons) > 5 || b.Type != "" {
				return errors.New("二级菜单最多 5 个，父菜单不能同时配置动作")
			}
			for _, sub := range b.SubButtons {
				if len(sub.SubButtons) > 0 {
					return errors.New("仅支持两级菜单")
				}
				if err := validateButton(sub, miniAppID, 8); err != nil {
					return err
				}
			}
		} else if err := validateButton(b, miniAppID, 4); err != nil {
			return err
		}
	}
	return nil
}
func validateButton(b Button, miniAppID string, maxName int) error {
	if b.Name == "" || len([]rune(b.Name)) > maxName {
		return errors.New("菜单名称为空或过长")
	}
	switch b.Type {
	case "click":
		if b.Key != "ORDERS_RECENT_1" && b.Key != "ORDERS_RECENT_3" {
			return errors.New("请选择最近一次或最近三次订单动作")
		}
	case "miniprogram":
		if miniAppID == "" || b.AppID != miniAppID {
			return errors.New("小程序 AppID 不匹配")
		}
		u, err := url.Parse(b.PagePath)
		if err != nil || u.Scheme != "" || u.Host != "" {
			return errors.New("小程序路径无效")
		}
		q := u.Query()
		ok := false
		switch u.Path {
		case "pages/price-list/price-list":
			ok = len(q) == 1 && len(q["entry"]) == 1 && len(q.Get("entry")) == 32
		case "pages/service/service":
			ok = len(q["key"]) == 1 && (len(q) == 1 || (len(q) == 2 && q.Get("key") == "orders" && len(q["source"]) == 1 && q.Get("source") == "official")) && (q.Get("key") == "orders" || q.Get("key") == "productOrder" || q.Get("key") == "directShip")
		case "pages/index/index":
			ok = len(q) == 0
		}
		if !ok {
			return errors.New("请选择豆单、订单或下单页面")
		}
		fallthrough
	case "view":
		u, err := url.Parse(b.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("备用网页必须是 HTTPS 地址")
		}
	case "media_id", "view_limited":
		if strings.TrimSpace(b.MediaID) == "" {
			return errors.New("素材 ID 不能为空")
		}
	default:
		return errors.New("不支持此菜单类型，请编辑后发布")
	}
	return nil
}
