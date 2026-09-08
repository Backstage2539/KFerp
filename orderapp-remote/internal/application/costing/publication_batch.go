package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// PublicationTableMetadata is frozen inside config_json.publication_batch.
// Legacy rows have no batch metadata and remain independent publications.
type PublicationTableMetadata struct {
	ReleaseID      string `json:"release_id,omitempty"`
	TableKey       string `json:"table_key,omitempty"`
	TableName      string `json:"table_name,omitempty"`
	IsDefaultTable bool   `json:"is_default_table"`
}

type BeanListBatchTable struct {
	Key                      string         `json:"key"`
	Name                     string         `json:"name"`
	Config                   map[string]any `json:"config"`
	Content                  map[string]any `json:"content"`
	PriceSourcePublicationID int64          `json:"price_source_publication_id,omitempty"`
	StyleSourcePublicationID int64          `json:"style_source_publication_id,omitempty"`
	SourceVersion            string         `json:"source_version,omitempty"`
}

type BeanListBatchCommand struct {
	PublishBeanListCommand
	DefaultTableKey string               `json:"default_table_key"`
	Tables          []BeanListBatchTable `json:"tables"`
}

type BeanListBatchResult struct {
	ReleaseID string                `json:"release_id"`
	Version   string                `json:"version"`
	Tables    []BeanListPublication `json:"tables"`
	PDFErrors map[int64]string      `json:"pdf_errors,omitempty"`
}

type beanListBatchRepository interface {
	SaveBeanListBatch(context.Context, []PublishBeanListCommand, bool) ([]BeanListPublication, error)
}

func BeanListBatchMetadata(config map[string]any) PublicationTableMetadata {
	meta := PublicationTableMetadata{IsDefaultTable: true}
	if raw, ok := config["publication_batch"]; ok {
		body, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(body, &meta)
		}
	}
	return meta
}

func SetBeanListBatchMetadata(config map[string]any, meta PublicationTableMetadata) {
	config["publication_batch"] = map[string]any{"release_id": meta.ReleaseID, "table_key": meta.TableKey, "table_name": meta.TableName, "is_default_table": meta.IsDefaultTable}
}

func copyBeanListMap(source map[string]any) (map[string]any, error) {
	if source == nil {
		return map[string]any{}, nil
	}
	body, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var target map[string]any
	err = json.Unmarshal(body, &target)
	return target, err
}

func (s *Service) PublishBeanListBatch(ctx context.Context, cmd BeanListBatchCommand) (*BeanListBatchResult, error) {
	return s.saveBeanListBatch(ctx, cmd, true)
}

func (s *Service) SaveBeanListDraftBatch(ctx context.Context, cmd BeanListBatchCommand) (*BeanListBatchResult, error) {
	return s.saveBeanListBatch(ctx, cmd, false)
}

func (s *Service) saveBeanListBatch(ctx context.Context, cmd BeanListBatchCommand, publish bool) (*BeanListBatchResult, error) {
	repo, ok := s.repo.(beanListBatchRepository)
	if !ok {
		return nil, fmt.Errorf("batch publication repository required")
	}
	if len(cmd.Tables) == 0 {
		return nil, fmt.Errorf("至少保留一张价格表")
	}
	defaultKey := strings.TrimSpace(cmd.DefaultTableKey)
	keys, names := map[string]bool{}, map[string]bool{}
	for _, table := range cmd.Tables {
		key, name := strings.TrimSpace(table.Key), strings.TrimSpace(table.Name)
		if key == "" || keys[key] {
			return nil, fmt.Errorf("价格表标识为空或重复")
		}
		if name == "" || names[name] {
			return nil, fmt.Errorf("价格表名称不能为空或重复：%s", name)
		}
		keys[key], names[name] = true, true
	}
	if !keys[defaultKey] {
		return nil, fmt.Errorf("请选择本版本的默认价格表")
	}
	commands := make([]PublishBeanListCommand, 0, len(cmd.Tables))
	for _, table := range cmd.Tables {
		item := cmd.PublishBeanListCommand
		var err error
		item.Config, err = copyBeanListMap(table.Config)
		if err != nil {
			return nil, err
		}
		item.Content, err = copyBeanListMap(table.Content)
		if err != nil {
			return nil, err
		}
		delete(item.Config, "publication_batch")
		item.PriceSourcePublicationID = table.PriceSourcePublicationID
		item.StyleSourcePublicationID = table.StyleSourcePublicationID
		item.SourceVersion = table.SourceVersion
		name := strings.TrimSpace(table.Name)
		item.Content["title"] = name
		item.Config["version"] = cmd.Version
		item.Config["changelog"] = cmd.Changelog
		item, err = normalizeBeanListCommand(item)
		if err == nil {
			err = s.validateProductSpecSelections(ctx, &item)
		}
		if err == nil && !beanListUsesConcreteProductSpecSelections(&item) {
			err = s.validatePriceTierTemplateUnitCompatibility(ctx, &item)
		}
		if err == nil {
			err = s.applyProductSalesUnitSnapshots(ctx, &item)
		}
		if err == nil && publish {
			if !beanListBatchHasContent(item.Content) {
				err = fmt.Errorf("请选择商品和规格，价格表不能为空")
			}
			if err == nil {
				err = validateBeanListBatchPrices(item)
				if err == nil {
					err = validateBeanListFinalPriceSnapshots(item)
				}
			}
		}
		if err == nil && publish {
			err = s.ValidateBeanListOrderability(ctx, item)
		}
		if err != nil {
			return nil, fmt.Errorf("价格表「%s」：%w", name, err)
		}
		SetBeanListBatchMetadata(item.Config, PublicationTableMetadata{TableKey: strings.TrimSpace(table.Key), TableName: name, IsDefaultTable: strings.TrimSpace(table.Key) == defaultKey})
		commands = append(commands, item)
	}
	rows, err := repo.SaveBeanListBatch(ctx, commands, publish)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(commands) {
		return nil, fmt.Errorf("价格表发布结果不完整")
	}
	for i := range rows {
		rows[i].PublicationTableMetadata = BeanListBatchMetadata(rows[i].Config)
	}
	return &BeanListBatchResult{ReleaseID: rows[0].ReleaseID, Version: rows[0].Version, Tables: rows}, nil
}

// NextBeanListPublicationVersion is also used under the repository scope lock.
func NextBeanListPublicationVersion(requested string, rows []BeanListPublication) string {
	return nextBeanListPublicationVersion(requested, rows)
}

func beanListBatchHasRows(value any) bool { rows, ok := value.([]any); return ok && len(rows) > 0 }

func beanListBatchHasContent(content map[string]any) bool {
	if beanListBatchHasRows(content["price_rows"]) {
		return true
	}
	groups, _ := content["groups"].([]any)
	for _, raw := range groups {
		if group, ok := raw.(map[string]any); ok && beanListBatchHasRows(group["items"]) {
			return true
		}
	}
	return false
}

func validateBeanListBatchPrices(cmd PublishBeanListCommand) error {
	if rows, ok := cmd.Content["price_rows"].([]any); ok && len(rows) > 0 {
		for i, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok || numberValue(row["final_unit_price"]) <= 0 {
				return fmt.Errorf("第%d个价格行缺少有效最终价，请重新生成预览", i+1)
			}
			if numberValue(row["product_id"]) <= 0 && numberValue(row["sku_id"]) <= 0 {
				return fmt.Errorf("第%d个价格行缺少商品身份", i+1)
			}
		}
		return nil
	}
	groups, _ := cmd.Content["groups"].([]any)
	for _, rawGroup := range groups {
		group, _ := rawGroup.(map[string]any)
		items, _ := group["items"].([]any)
		for i, raw := range items {
			item, _ := raw.(map[string]any)
			hasPrice := false
			for _, key := range []string{"commercial_wholesale_tiers", "green_bean_sale_tiers", "retail_bean_tiers", "drip_wholesale_tiers"} {
				tiers, _ := item[key].([]any)
				for _, rawTier := range tiers {
					tier, _ := rawTier.(map[string]any)
					hasPrice = hasPrice || numberValue(tier["final_unit_price"]) > 0
				}
			}
			if !hasPrice {
				return fmt.Errorf("商品「%s」（第%d项）缺少发布报价，请重新生成预览", stringValue(item["name"]), i+1)
			}
		}
	}
	return nil
}
