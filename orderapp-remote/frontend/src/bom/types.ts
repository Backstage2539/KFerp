export interface BomRow {
  product_id: number
  product: string
  yield_rate: number
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
  yield_rate: number
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
