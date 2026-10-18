// Package specconversion prices an explicit specification relation. Names and
// unit labels never supply a conversion factor.
package specconversion

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

type Member struct {
	SpecID        int64  `json:"bom_spec_id,omitempty"`
	TemplateID    int64  `json:"template_id,omitempty"`
	TemplateKey   string `json:"template_key,omitempty"`
	Label         string `json:"label"`
	InventoryUnit string `json:"inventory_unit"`
	BaseQuantity  string `json:"base_quantity"`
}
type Identity struct {
	SpecID        int64
	TemplateID    int64
	TemplateKey   string
	InventoryUnit string
}
type Group struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	BaseUnit string   `json:"base_unit"`
	Revision int64    `json:"revision"`
	Active   bool     `json:"active"`
	Members  []Member `json:"members"`
}

var decimal = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,10})?$`)

func positive(value string) (*big.Rat, error) {
	if !decimal.MatchString(value) {
		return nil, fmt.Errorf("换算数量和价格必须是有效正数（最多10位小数）")
	}
	r, ok := new(big.Rat).SetString(value)
	if !ok || r.Sign() <= 0 {
		return nil, fmt.Errorf("换算数量和价格必须大于0")
	}
	return r, nil
}

func (g Group) Validate() error {
	if strings.TrimSpace(g.Name) == "" || strings.TrimSpace(g.BaseUnit) == "" || len(g.Name) > 240 || len(g.Members) < 2 || len(g.Members) > 200 {
		return fmt.Errorf("请填写换算组名称、基准单位和至少两个规格关系（最多200个）")
	}
	seen := map[string]bool{}
	for _, m := range g.Members {
		if m.SpecID < 0 || m.TemplateID < 0 || (m.SpecID > 0) == (m.TemplateID > 0) || (m.TemplateID > 0 && strings.TrimSpace(m.TemplateKey) == "") || (m.SpecID > 0 && m.TemplateKey != "") || strings.TrimSpace(m.InventoryUnit) == "" {
			return fmt.Errorf("请选择明确的商品规格或模板规格及库存单位")
		}
		if _, err := positive(m.BaseQuantity); err != nil {
			return err
		}
		key := fmt.Sprintf("%d:%d:%s:%s", m.SpecID, m.TemplateID, m.TemplateKey, strings.ToLower(m.InventoryUnit))
		if seen[key] {
			return fmt.Errorf("同一规格不能配置多个换算关系")
		}
		seen[key] = true
	}
	return nil
}

func (g Group) Match(spec Identity) (Member, error) {
	var found *Member
	for _, m := range g.Members {
		if !strings.EqualFold(m.InventoryUnit, spec.InventoryUnit) {
			continue
		}
		matches := m.SpecID > 0 && m.SpecID == spec.SpecID
		if m.TemplateID > 0 {
			matches = m.TemplateID == spec.TemplateID && m.TemplateKey == spec.TemplateKey
		}
		if !matches {
			continue
		}
		if found != nil {
			return Member{}, fmt.Errorf("规格存在冲突的换算关系，请调整换算组")
		}
		copy := m
		found = &copy
	}
	if found == nil {
		return Member{}, fmt.Errorf("规格没有明确关联此换算组，请补齐换算关系")
	}
	return *found, nil
}

// Convert rounds the rational result once, half up to cents. The order then
// multiplies this rounded per-target-unit price by the target quantity.
func Convert(price, source, target string) (string, error) {
	p, err := positive(price)
	if err != nil {
		return "", err
	}
	s, err := positive(source)
	if err != nil {
		return "", err
	}
	t, err := positive(target)
	if err != nil {
		return "", err
	}
	v := new(big.Rat).Quo(new(big.Rat).Mul(p, t), s)
	v.Mul(v, big.NewRat(100, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(v.Num(), v.Denom(), remainder)
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(v.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if quotient.Sign() <= 0 || !quotient.IsInt64() || quotient.Int64() > 999999999999 {
		return "", fmt.Errorf("换算单价超出有效金额范围")
	}
	cents := quotient.Int64()
	return fmt.Sprintf("%d.%02d", cents/100, cents%100), nil
}
