package manufacturing

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RouteExecution struct {
	Version          int    `json:"version"`
	NodeKey          string `json:"node_key"`
	ReceiptEnabled   bool   `json:"receipt_enabled"`
	ReceiptWarehouse string `json:"receipt_warehouse"`
	SourceWarehouse  string `json:"source_warehouse"`
	OutputUnit       string `json:"output_unit"`
}

func NormalizeRouteExecution(raw string, last bool, fallbackKey string) (string, error) {
	config := RouteExecution{Version: 1, NodeKey: fallbackKey, ReceiptEnabled: last, SourceWarehouse: "wip"}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return "", fmt.Errorf("工序完成配置无效")
		}
	}
	if config.Version != 1 {
		return "", fmt.Errorf("不支持的工序完成配置版本")
	}
	config.NodeKey = strings.TrimSpace(config.NodeKey)
	if config.NodeKey == "" {
		config.NodeKey = fallbackKey
	}
	config.SourceWarehouse = strings.TrimSpace(config.SourceWarehouse)
	if config.SourceWarehouse == "" {
		config.SourceWarehouse = "wip"
	}
	config.ReceiptWarehouse = strings.TrimSpace(config.ReceiptWarehouse)
	if config.ReceiptEnabled && !last && config.ReceiptWarehouse == "" {
		return "", fmt.Errorf("请指定中间工序完成后的入库位置")
	}
	if last && !config.ReceiptEnabled {
		return "", fmt.Errorf("末道工序需启用完成后入库")
	}
	config.OutputUnit = strings.TrimSpace(config.OutputUnit)
	if !last && config.OutputUnit == "" {
		config.OutputUnit = "g"
	}
	encoded, _ := json.Marshal(config)
	return string(encoded), nil
}
