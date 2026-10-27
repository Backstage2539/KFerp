// Package orderprogress defines the shared read-only fulfillment status.
package orderprogress

import "strings"

type Progress struct {
	Label       string `json:"label"`
	Production  string `json:"production"`
	Inbound     string `json:"inbound"`
	Stock       string `json:"stock"`
	CanShip     bool   `json:"can_ship"`
	BlockReason string `json:"block_reason"`
}
type Input struct {
	DeliveryMode, DeliveryStatus, FulfillmentMode, ProcessStatus, ShipStatus, ShipMethod, ConfirmationStatus string
	IsVoid, StockReady                                                                                       bool
}

func Compute(in Input) Progress {
	p := Progress{Production: in.ProcessStatus}
	if in.IsVoid {
		p.Label = "已作废"
		p.BlockReason = p.Label
		return p
	}
	if in.ConfirmationStatus != "" && in.ConfirmationStatus != "accepted" {
		p.Label = "待确认"
		p.BlockReason = "订单尚未确认"
		return p
	}
	if in.DeliveryMode == "customer_stock" {
		p.Inbound = map[string]string{"received": "已入库", "partially_received": "部分入库", "pending_receipt": "待入库"}[in.DeliveryStatus]
		if p.Inbound == "" {
			p.Inbound = "待入库"
		}
		if p.Production == "" {
			p.Production = "待生产"
		}
		p.Label = p.Production + " · " + p.Inbound
		p.BlockReason = "留厂订单无需发货"
		return p
	}
	if strings.Contains(in.ShipStatus, "已发货") || strings.Contains(in.ShipStatus, "部分发货") || in.ShipStatus == "已签收" || in.ShipStatus == "已出库" || in.ShipStatus == "已收货" || in.ShipStatus == "已完成" {
		p.Label = in.ShipStatus
		p.BlockReason = "请通过原发货记录处理"
		return p
	}
	ready := strings.Contains(in.ProcessStatus, "生产完成") || in.ProcessStatus == "无需生产" || in.ProcessStatus == "库存待发货"
	if in.FulfillmentMode == "stock" {
		p.Stock = "库存不足"
		p.Label = "库存不足 · 待入库"
		p.BlockReason = "本单实物库存尚未全部分配或批次不可用"
		if in.StockReady {
			p.Stock = "库存充足"
			p.Label = "库存充足 · 可以发货"
			p.CanShip = true
			p.BlockReason = ""
		}
	} else {
		p.Label = strings.Trim(strings.Join([]string{in.ProcessStatus, in.ShipStatus}, " · "), " ·")
		p.CanShip = ready
		if !ready {
			p.BlockReason = "订单尚未满足发货条件"
		}
	}
	if in.ShipMethod == "pickup" || in.ShipMethod == "self_pickup" || in.ShipMethod == "local_delivery" {
		p.CanShip = false
		p.BlockReason = "自提或本地送货不使用快递发货"
		if in.FulfillmentMode == "stock" && in.StockReady {
			p.Label = "库存充足 · 待交付"
		}
	}
	return p
}
