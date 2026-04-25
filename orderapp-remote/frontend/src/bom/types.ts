export interface BomListItem {
  product_id: number
  product: string
  roast_level: string
  yield_rate: number
  item_count: number
  updated_at: string
}

export interface BomItemRow {
  id: number
  material_id: number
  material_name: string
  ratio_pct: number
}

export interface BomDetail {
  product_id: number
  product_name: string
  roast_level: string
  yield_rate: number
  items: BomItemRow[]
  total_ratio: number
  updated_at: string
}

export interface Option {
  id: number
  name: string
}

export interface SaveBomRequest {
  product_id: number
}

export interface SaveBomItemRequest {
  product_id: number
  material_id: number
  ratio_pct: number
}

export interface DeleteBomItemRequest {
  product_id: number
  id: number
}

export interface BagSpecMapping {
  spec_g: number
  material_id: number
  material_name: string
}

export interface SaveBagSpecMappingRequest {
  spec_g: number
  material_id: number
}

export interface DeleteBagSpecMappingRequest {
  spec_g: number
}
