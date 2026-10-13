package pageentry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid     = errors.New("页面配置无效，请检查名称、内容和目标")
	ErrUnavailable = errors.New("页面未发布、已停用或内容不可用")
	ErrConflict    = errors.New("页面已被修改，请刷新后重试")
	ErrDenied      = errors.New("当前客户没有此页面的访问权限")
)

type Block struct {
	Kind    string `json:"kind"`
	Text    string `json:"text,omitempty"`
	AssetID string `json:"asset_id,omitempty"`
	Caption string `json:"caption,omitempty"`
}
type Document struct {
	BeanCenter    bool    `json:"bean_center"`
	BeanSort      int     `json:"bean_sort"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Visibility    string  `json:"visibility"`
	PublicationID int64   `json:"publication_id,omitempty"`
	Target        string  `json:"target,omitempty"`
	Blocks        []Block `json:"blocks,omitempty"`
}
type Entry struct {
	Key               string    `json:"key"`
	Draft             Document  `json:"draft"`
	Published         *Document `json:"published,omitempty"`
	Revision          int64     `json:"revision"`
	PublishedRevision int64     `json:"published_revision"`
	Enabled           bool      `json:"enabled"`
	HasDraft          bool      `json:"has_draft"`
	Deleted           bool      `json:"deleted"`
	UpdatedAt         time.Time `json:"updated_at"`
}
type Target struct {
	ID         int64  `json:"id"`
	TableScope string `json:"table_scope"`
	Name       string `json:"name"`
	TypeName   string `json:"type_name"`
	OwnerType  string `json:"owner_type"`
	OwnerKey   string `json:"owner_key"`
	Version    string `json:"version"`
}
type Reference struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Name   string `json:"name"`
}
type Function struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Public bool   `json:"public"`
}

var Functions = []Function{
	{"home", "首页", "pages/index/index", true},
	{"beans", "豆单中心", "pages/bean-list-center/bean-list-center", true},
	{"customer", "客户中心", "pages/home/home", false},
	{"orders", "全部订单", "pages/service/service?key=orders&source=official", false},
	{"order", "商品下单", "pages/service/service?key=productOrder", false},
	{"direct_ship", "一件代发", "pages/service/service?key=directShip", false},
	{"mall", "商城", "pages/mall/mall", true},
	{"profile", "个人中心", "pages/profile/profile", false},
}

func FunctionPath(key string) string {
	for _, f := range Functions {
		if f.Key == key {
			return f.Path
		}
	}
	return ""
}
func NewKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func ValidKey(key string) bool {
	if len(key) != 32 {
		return false
	}
	b, err := hex.DecodeString(key)
	return err == nil && len(b) == 16 && key == strings.ToLower(key)
}
func MiniPath(key string) string { return "pages/page-entry/page-entry?entry=" + key }
func (d Document) HasAsset(key string) bool {
	for _, b := range d.Blocks {
		if b.Kind == "image" && b.AssetID == key {
			return true
		}
	}
	return false
}
func Normalize(d Document) Document {
	d.Name = strings.TrimSpace(d.Name)
	switch d.Kind {
	case "price":
		d.Target = ""
		d.Blocks = nil
	case "function":
		d.PublicationID = 0
		d.Blocks = nil
	case "article":
		d.Target = ""
		d.PublicationID = 0
	}
	if d.Visibility == "" {
		d.Visibility = "authenticated"
	}
	return d
}
func Validate(d Document, publish bool) error {
	if strings.TrimSpace(d.Name) == "" || utf8.RuneCountInString(d.Name) > 100 || (d.Visibility != "public" && d.Visibility != "authenticated" && d.Visibility != "registered") {
		return ErrInvalid
	}
	if d.Kind != "price" && (d.Visibility == "registered" || d.BeanCenter) {
		return ErrInvalid
	}
	if d.BeanSort < 0 || d.BeanSort > 99999 {
		return ErrInvalid
	}
	switch d.Kind {
	case "price":
		if d.PublicationID < 0 || publish && d.PublicationID == 0 {
			return ErrInvalid
		}
	case "function":
		if d.Target == "" && !publish {
			return nil
		}
		found := false
		for _, f := range Functions {
			if f.Key == d.Target {
				found = true
				if d.Visibility == "public" && !f.Public {
					return ErrDenied
				}
			}
		}
		if !found {
			return ErrInvalid
		}
	case "article":
		if len(d.Blocks) > 100 || publish && len(d.Blocks) == 0 {
			return ErrInvalid
		}
		total := 0
		for _, b := range d.Blocks {
			total += len(b.Text) + len(b.Caption)
			switch b.Kind {
			case "heading", "text":
				if publish && strings.TrimSpace(b.Text) == "" {
					return ErrInvalid
				}
			case "image":
				if !ValidKey(b.AssetID) {
					return ErrInvalid
				}
			default:
				return ErrInvalid
			}
		}
		if total > 100000 {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
