package costing

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domain "orderapp/internal/domain/costing"
)

type BeanListPublicationCopyCommand struct {
	SourceQuery         BeanListPublicationQuery `json:"source_query"`
	SourcePublicationID int64                    `json:"source_publication_id"`
	CustomerID          int64                    `json:"customer_id"`
	TargetTableKey      string                   `json:"target_table_key"`
	CopyRequestID       string                   `json:"copy_request_id,omitempty"`
	Batch               BeanListBatchCommand     `json:"batch,omitempty"`
}

type BeanListPublicationCopyStats struct {
	SourcePublicationID int64  `json:"source_publication_id"`
	SourceVersion       string `json:"source_version"`
	SourceProductCount  int    `json:"source_product_count"`
	SourceSpecCount     int    `json:"source_spec_count"`
	CopyProductCount    int    `json:"copy_product_count"`
	CopySpecCount       int    `json:"copy_spec_count"`
	SkipProductCount    int    `json:"skip_product_count"`
	SkipSpecCount       int    `json:"skip_spec_count"`
}

type BeanListPublicationCopyResult struct {
	Stats BeanListPublicationCopyStats `json:"copy_stats"`
	Draft *BeanListBatchResult         `json:"draft,omitempty"`
}

type BeanListCopyProductIdentity struct {
	ProductID           int64
	ParentProductID     int64
	SKUID               int64
	BOMSpecID           int64
	BOMVariantID        int64
	CustomerID          int64
	AliasID             int64
	DisplayName         string
	ItemCode            string
	BrandName           string
	DisplayCategoryName string
}

type beanListCopyAllowedProduct struct {
	identity BeanListCopyProductIdentity
	keys     map[string]bool
}

func (s *Service) PreviewBeanListPublicationCopy(ctx context.Context, cmd BeanListPublicationCopyCommand) (BeanListPublicationCopyStats, error) {
	_, stats, err := s.prepareBeanListPublicationCopy(ctx, cmd, false)
	return stats, err
}

func (s *Service) CopyBeanListPublicationToDraft(ctx context.Context, cmd BeanListPublicationCopyCommand) (BeanListPublicationCopyResult, error) {
	if existing, found, err := s.existingBeanListPublicationCopy(ctx, cmd); err != nil {
		return BeanListPublicationCopyResult{}, err
	} else if found {
		return existing, nil
	}
	batch, stats, err := s.prepareBeanListPublicationCopy(ctx, cmd, true)
	if err != nil {
		return BeanListPublicationCopyResult{}, err
	}
	if stats.CopySpecCount == 0 {
		return BeanListPublicationCopyResult{}, fmt.Errorf("当前客户没有与来源价格表相同的有效商品或规格；原价格表未覆盖")
	}
	draft, err := s.SaveBeanListDraftBatch(ctx, batch)
	if err != nil {
		return BeanListPublicationCopyResult{}, err
	}
	return BeanListPublicationCopyResult{Stats: stats, Draft: draft}, nil
}

func (s *Service) prepareBeanListPublicationCopy(ctx context.Context, cmd BeanListPublicationCopyCommand, includeBatch bool) (BeanListBatchCommand, BeanListPublicationCopyStats, error) {
	stats := BeanListPublicationCopyStats{SourcePublicationID: cmd.SourcePublicationID}
	if s.repo == nil || cmd.SourcePublicationID <= 0 || cmd.CustomerID <= 0 || strings.TrimSpace(cmd.TargetTableKey) == "" {
		return BeanListBatchCommand{}, stats, fmt.Errorf("来源价格表、客户和目标表标识不能为空")
	}
	query, err := normalizeBeanListPublicationQuery(cmd.SourceQuery)
	if err != nil {
		return BeanListBatchCommand{}, stats, err
	}
	if query.PublicationPurpose != BeanListPublicationPurposeFactorySupply || (query.OwnerType != "official" && query.OwnerType != "customer") {
		return BeanListBatchCommand{}, stats, fmt.Errorf("来源必须是有权限查看的已发布供货价格表")
	}
	source, err := s.repo.LoadBeanListPublication(ctx, query, cmd.SourcePublicationID)
	if err != nil {
		return BeanListBatchCommand{}, stats, err
	}
	if source == nil || source.Status != "published" || source.PublicationPurpose != BeanListPublicationPurposeFactorySupply || (source.OwnerType != "official" && source.OwnerType != "customer") {
		return BeanListBatchCommand{}, stats, fmt.Errorf("来源必须是已发布的公共或客户供货价格表；草稿、撤回、归档和客户转售表不可复制")
	}
	stats.SourceVersion = source.Version
	if strings.TrimSpace(cmd.Batch.ListType) != "" && !sameBeanListType(source.ListType, cmd.Batch.ListType) {
		return BeanListBatchCommand{}, stats, fmt.Errorf("来源价格表类型与当前价格表不一致")
	}
	if source.ClassificationTemplateID > 0 && cmd.Batch.ClassificationTemplateID > 0 && source.ClassificationTemplateID != cmd.Batch.ClassificationTemplateID {
		return BeanListBatchCommand{}, stats, fmt.Errorf("来源商品分类与当前价格表不一致")
	}
	inputs, err := s.customerProductInputsForCopy(ctx, cmd.CustomerID)
	if err != nil {
		return BeanListBatchCommand{}, stats, err
	}
	allowed := make([]beanListCopyAllowedProduct, 0, len(inputs))
	for _, input := range inputs {
		identity := BeanListCopyProductIdentity{
			ProductID: input.ProductID, ParentProductID: firstPositiveID(input.EffectiveParentProductID, input.ParentProductID),
			SKUID: input.SKUID, BOMSpecID: input.BomSpecID, BOMVariantID: input.BomVariantID,
			CustomerID: cmd.CustomerID, AliasID: input.CustomerProductAliasID,
			DisplayName: firstBeanListCopyText(input.CustomerProductDisplayName, input.Name), ItemCode: input.CustomerItemCode,
			BrandName: input.BrandName, DisplayCategoryName: input.DisplayCategoryName,
		}
		if identity.CustomerID != cmd.CustomerID && identity.AliasID <= 0 {
			continue
		}
		if identity.ParentProductID <= 0 {
			identity.ParentProductID = identity.ProductID
		}
		keys := beanListCopyIdentityKeys(identity)
		if len(keys) > 0 {
			allowed = append(allowed, beanListCopyAllowedProduct{identity: identity, keys: keys})
		}
	}
	config, err := copyBeanListMap(source.Config)
	if err != nil {
		return BeanListBatchCommand{}, stats, err
	}
	content, err := copyBeanListMap(source.Content)
	if err != nil {
		return BeanListBatchCommand{}, stats, err
	}
	config, content, stats = filterBeanListPublicationCopy(config, content, allowed, stats)
	if !includeBatch {
		return BeanListBatchCommand{}, stats, nil
	}
	batch := cmd.Batch
	if batch.PublicationPurpose != "" && batch.PublicationPurpose != BeanListPublicationPurposeFactorySupply {
		return BeanListBatchCommand{}, stats, fmt.Errorf("只能复制到客户供货价格表")
	}
	if batch.OwnerType != "customer" || batch.OwnerKey != strconv.FormatInt(cmd.CustomerID, 10) {
		return BeanListBatchCommand{}, stats, fmt.Errorf("目标价格表归属不一致")
	}
	if !sameBeanListType(batch.ListType, source.ListType) {
		return BeanListBatchCommand{}, stats, fmt.Errorf("来源价格表类型与当前价格表不一致")
	}
	targetIndex := -1
	for i := range batch.Tables {
		if strings.TrimSpace(batch.Tables[i].Key) == strings.TrimSpace(cmd.TargetTableKey) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return BeanListBatchCommand{}, stats, fmt.Errorf("当前目标价格表不存在，请刷新后重试")
	}
	batch.Tables[targetIndex].Config = config
	batch.Tables[targetIndex].Content = content
	batch.Tables[targetIndex].PriceSourcePublicationID = 0
	batch.Tables[targetIndex].StyleSourcePublicationID = 0
	batch.Tables[targetIndex].SourceVersion = ""
	if requestID := strings.TrimSpace(cmd.CopyRequestID); requestID != "" {
		table := &batch.Tables[targetIndex]
		table.CopyRequestID = requestID
		table.CopyActor = strings.TrimSpace(batch.Actor)
		table.CopySourcePublicationID = source.ID
		table.CopySourceVersion = source.Version
		table.CopySourceProductCount = stats.SourceProductCount
		table.CopySourceSpecCount = stats.SourceSpecCount
		table.CopyProductCount = stats.CopyProductCount
		table.CopySpecCount = stats.CopySpecCount
		table.CopySkipProductCount = stats.SkipProductCount
		table.CopySkipSpecCount = stats.SkipSpecCount
	}
	batch.CustomerID = cmd.CustomerID
	batch.PublicationPurpose = BeanListPublicationPurposeFactorySupply
	return batch, stats, nil
}

func (s *Service) existingBeanListPublicationCopy(ctx context.Context, cmd BeanListPublicationCopyCommand) (BeanListPublicationCopyResult, bool, error) {
	requestID := strings.TrimSpace(cmd.CopyRequestID)
	if requestID == "" || s.repo == nil {
		return BeanListPublicationCopyResult{}, false, nil
	}
	query := BeanListPublicationQuery{
		ListType: cmd.Batch.ListType, PublicationPurpose: BeanListPublicationPurposeFactorySupply,
		ProductTypeCategoryID: cmd.Batch.ProductTypeCategoryID, ClassificationTemplateID: cmd.Batch.ClassificationTemplateID,
		OwnerType: "customer", OwnerKey: strconv.FormatInt(cmd.CustomerID, 10), Scope: "customer", CustomerID: cmd.CustomerID,
	}
	rows, err := s.repo.ListBeanListPublications(ctx, query)
	if err != nil {
		return BeanListPublicationCopyResult{}, false, err
	}
	var target *BeanListPublication
	for i := range rows {
		meta := rows[i].PublicationTableMetadata
		if rows[i].Status == "draft" && meta.TableKey == strings.TrimSpace(cmd.TargetTableKey) && meta.CopyRequestID == requestID &&
			meta.CopyActor == strings.TrimSpace(cmd.Batch.Actor) && meta.CopySourcePublicationID == cmd.SourcePublicationID {
			target = &rows[i]
			break
		}
	}
	if target == nil {
		return BeanListPublicationCopyResult{}, false, nil
	}
	batchRows := make([]BeanListPublication, 0, len(rows))
	for _, row := range rows {
		if row.Status == "draft" && row.ReleaseID == target.ReleaseID {
			batchRows = append(batchRows, row)
		}
	}
	meta := target.PublicationTableMetadata
	stats := BeanListPublicationCopyStats{
		SourcePublicationID: meta.CopySourcePublicationID, SourceVersion: meta.CopySourceVersion,
		SourceProductCount: meta.CopySourceProductCount, SourceSpecCount: meta.CopySourceSpecCount,
		CopyProductCount: meta.CopyProductCount, CopySpecCount: meta.CopySpecCount,
		SkipProductCount: meta.CopySkipProductCount, SkipSpecCount: meta.CopySkipSpecCount,
	}
	return BeanListPublicationCopyResult{Stats: stats, Draft: &BeanListBatchResult{ReleaseID: target.ReleaseID, Version: target.Version, Tables: batchRows}}, true, nil
}

func (s *Service) customerProductInputsForCopy(ctx context.Context, customerID int64) ([]domain.ProductInput, error) {
	repo, ok := s.repo.(customerScopedProductInputRepository)
	if !ok {
		return nil, fmt.Errorf("客户商品规格查询不可用，未覆盖当前价格表")
	}
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	return repo.LoadProductInputsForCustomer(ctx, params, customerID)
}

func beanListCopyIdentityKeys(identity BeanListCopyProductIdentity) map[string]bool {
	parentID := firstPositiveID(identity.ParentProductID, identity.ProductID)
	keys := map[string]bool{}
	if parentID <= 0 {
		return keys
	}
	if identity.BOMSpecID > 0 && identity.BOMVariantID > 0 {
		keys[fmt.Sprintf("%d:bom:%d:%d", parentID, identity.BOMSpecID, identity.BOMVariantID)] = true
	} else if identity.SKUID > 0 {
		keys[fmt.Sprintf("%d:sku:%d", parentID, identity.SKUID)] = true
	} else {
		keys[fmt.Sprintf("%d:product", parentID)] = true
	}
	return keys
}

func beanListCopyIdentityKey(row map[string]any) (string, int64) {
	parentID := int64(numberValue(row["parent_product_id"]))
	if parentID <= 0 {
		parentID = int64(numberValue(row["effective_parent_product_id"]))
	}
	if parentID <= 0 {
		parentID = int64(numberValue(row["product_id"]))
	}
	if parentID <= 0 {
		parentID = int64(numberValue(row["productId"]))
	}
	if parentID <= 0 {
		return "", 0
	}
	specID := int64(numberValue(row["bom_spec_id"]))
	variantID := int64(numberValue(row["bom_variant_id"]))
	if specID > 0 && variantID > 0 {
		return fmt.Sprintf("%d:bom:%d:%d", parentID, specID, variantID), parentID
	}
	skuID := int64(numberValue(row["sku_id"]))
	if skuID <= 0 {
		productID := int64(numberValue(row["product_id"]))
		if productID > 0 && productID != parentID {
			skuID = productID
		}
	}
	if skuID > 0 {
		return fmt.Sprintf("%d:sku:%d", parentID, skuID), parentID
	}
	return fmt.Sprintf("%d:product", parentID), parentID
}

func filterBeanListPublicationCopy(config, content map[string]any, allowed []beanListCopyAllowedProduct, stats BeanListPublicationCopyStats) (map[string]any, map[string]any, BeanListPublicationCopyStats) {
	allowedByKey := map[string]beanListCopyAllowedProduct{}
	for _, candidate := range allowed {
		for key := range candidate.keys {
			allowedByKey[key] = candidate
		}
	}
	sourceSpecs, copiedSpecs := map[string]int64{}, map[string]int64{}
	keptIDs := map[string]bool{}
	filterRows := func(rows []any) []any {
		out := make([]any, 0, len(rows))
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			key, productID := beanListCopyIdentityKey(row)
			if key == "" {
				continue
			}
			sourceSpecs[key] = productID
			candidate, ok := allowedByKey[key]
			if !ok {
				continue
			}
			copiedSpecs[key] = productID
			keptIDs[strconv.FormatInt(candidate.identity.ProductID, 10)] = true
			keptIDs[strconv.FormatInt(candidate.identity.SKUID, 10)] = true
			copyBeanListCustomerIdentity(row, candidate.identity)
			row["frozen_final_price"] = true
			out = append(out, row)
		}
		return out
	}
	if rows, ok := beanListCopyAnySlice(content["price_rows"]); ok {
		content["price_rows"] = filterRows(rows)
	}
	groups, _ := beanListCopyAnySlice(content["groups"])
	filteredGroups := make([]any, 0, len(groups))
	for _, raw := range groups {
		group, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		items, _ := beanListCopyAnySlice(group["items"])
		filteredItems := filterRows(items)
		if len(filteredItems) == 0 {
			continue
		}
		group["items"] = filteredItems
		filteredGroups = append(filteredGroups, group)
	}
	if _, exists := content["groups"]; exists {
		content["groups"] = filteredGroups
	}
	filterMapRows := func(value any) []any {
		rows, ok := beanListCopyAnySlice(value)
		if !ok {
			return nil
		}
		out := make([]any, 0, len(rows))
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			key, productID := beanListCopyIdentityKey(row)
			if key == "" {
				continue
			}
			sourceSpecs[key] = productID
			if _, ok := allowedByKey[key]; !ok {
				continue
			}
			copiedSpecs[key] = productID
			out = append(out, row)
		}
		return out
	}
	if rows, exists := config["product_spec_selections"]; exists {
		config["product_spec_selections"] = filterMapRows(rows)
	}
	if rows, exists := content["product_spec_selections"]; exists {
		content["product_spec_selections"] = filterMapRows(rows)
	}
	if rows, ok := beanListCopyAnySlice(config["selectedProductIDs"]); ok {
		selected := make([]any, 0, len(rows))
		for _, raw := range rows {
			if keptIDs[fmt.Sprint(raw)] || keptIDs[strconv.FormatInt(int64(numberValue(raw)), 10)] {
				selected = append(selected, raw)
			}
		}
		config["selectedProductIDs"] = selected
	}
	if len(copiedSpecs) > 0 {
		products := map[int64]bool{}
		for _, productID := range copiedSpecs {
			products[productID] = true
		}
		stats.CopyProductCount = len(products)
		stats.CopySpecCount = len(copiedSpecs)
	}
	if len(sourceSpecs) > 0 {
		products := map[int64]bool{}
		for _, productID := range sourceSpecs {
			products[productID] = true
		}
		stats.SourceProductCount = len(products)
		stats.SourceSpecCount = len(sourceSpecs)
	}
	stats.SkipProductCount = stats.SourceProductCount - stats.CopyProductCount
	stats.SkipSpecCount = stats.SourceSpecCount - stats.CopySpecCount
	if _, exists := content["totalItems"]; exists {
		content["totalItems"] = stats.CopyProductCount
	}
	return config, content, stats
}

func copyBeanListCustomerIdentity(row map[string]any, identity BeanListCopyProductIdentity) {
	row["customer_id"] = identity.CustomerID
	row["customer_product_alias_id"] = identity.AliasID
	if name := strings.TrimSpace(identity.DisplayName); name != "" {
		row["customer_product_display_name"] = name
		row["customer_product_display_name_snapshot"] = name
		row["display_name_snapshot"] = name
		row["name"] = name
	}
	row["customer_item_code"] = identity.ItemCode
	row["customer_item_code_snapshot"] = identity.ItemCode
	row["brand_name"] = identity.BrandName
	row["brand_name_snapshot"] = identity.BrandName
	row["display_category_name"] = identity.DisplayCategoryName
	row["display_category_snapshot"] = identity.DisplayCategoryName
}

func sameBeanListType(left, right string) bool {
	l, leftErr := normalizeBeanListType(left)
	r, rightErr := normalizeBeanListType(right)
	return leftErr == nil && rightErr == nil && l == r
}

func firstBeanListCopyText(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func beanListCopyAnySlice(value any) ([]any, bool) {
	rows, ok := value.([]any)
	return rows, ok
}

func firstPositiveID(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
