package orderresolution

import "fmt"

type Policy struct {
	Disabled  map[string][]string `json:"disabled"`
	Approvals map[string]string   `json:"approvals"`
}

func (p Policy) Validate() error {
	options := Options()
	for kind, values := range p.Disabled {
		if kind != "action" && !oneOf(kind, "production", "direct_ship", "customer_stock", "stock", "stock_reservation", "fee", "funds") {
			return fmt.Errorf("不支持的处置类别：%s", kind)
		}
		for _, value := range values {
			found := false
			for _, o := range options[kind] {
				if o.Value == value {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("不支持的处置选项：%s", value)
			}
		}
	}
	for action, permission := range p.Approvals {
		found := false
		for _, o := range options["action"] {
			if o.Value == action {
				found = true
			}
		}
		if !found || !oneOf(permission, "orders.resolve", "production.resolve", "stock.resolve", "finance.resolve") {
			return fmt.Errorf("审批操作或确认岗位无效")
		}
	}
	return nil
}
func (p Policy) Allows(kind, value string) bool { return !oneOf(value, p.Disabled[kind]...) }
func Preview(c Context, cmd Command) (Plan, error) {
	for _, key := range []string{"action:" + cmd.Action, "posting_mode:" + cmd.PostingMode} {
		if reason := c.Unavailable[key]; reason != "" {
			return Plan{}, fmt.Errorf("%s", reason)
		}
	}
	if err := c.Policy.Validate(); err != nil {
		return Plan{}, err
	}
	if !c.Policy.Allows("action", cmd.Action) {
		return Plan{}, fmt.Errorf("管理员已停用此订单操作")
	}
	for _, d := range cmd.Dispositions {
		if reason := c.Unavailable[d.Kind+":"+d.Choice]; reason != "" {
			return Plan{}, fmt.Errorf("%s", reason)
		}
		if !c.Policy.Allows(d.Kind, d.Choice) {
			return Plan{}, fmt.Errorf("管理员已停用所选处置，请重新选择")
		}
	}
	p, err := preview(c, cmd)
	if err != nil {
		return p, err
	}
	if permission := c.Policy.Approvals[cmd.Action]; permission != "" {
		p.Tasks = append(p.Tasks, Task{Kind: "approval", Choice: "approve", Label: "订单处理审批", Permission: permission})
	}
	return p, nil
}
func OptionsFor(c Context) map[string][]Option {
	options := Options()
	for kind, list := range options {
		for n := range list {
			if reason := c.Unavailable[kind+":"+list[n].Value]; reason != "" {
				list[n].Disabled = true
				list[n].Reason = reason
			}
			if kind == "action" && c.FulfillmentIssue != "" && !oneOf(list[n].Value, "hold", "resume") {
				list[n].Disabled = true
				list[n].Reason = c.FulfillmentIssue
			}
			if kind == "funds" && c.CustomerID <= 0 && list[n].Value != "refund" {
				list[n].Disabled = true
				list[n].Reason = "请先核对客户档案身份"
			}
			if !c.Policy.Allows(kind, list[n].Value) {
				list[n].Disabled = true
				list[n].Reason = "管理员已停用"
			}
		}
	}
	return options
}
