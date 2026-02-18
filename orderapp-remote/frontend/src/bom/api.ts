import axios from 'axios'
import type { BomListItem, BomDetail, Option, SaveBomRequest, SaveBomItemRequest, DeleteBomItemRequest, BagSpecMapping, SaveBagSpecMappingRequest, DeleteBagSpecMappingRequest } from './types'

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

export const bomApi = {
  // 获取 BOM 列表
  getBomList: (): Promise<BomListItem[]> =>
    api.get('/bom/list').then(res => res.data),

  // 获取 BOM 详情
  getBomDetail: (productId: number): Promise<BomDetail> =>
    api.get(`/bom/detail/${productId}`).then(res => res.data),

  // 获取产品选项
  getProducts: (): Promise<Option[]> =>
    api.get('/bom/products').then(res => res.data),

  // 获取所有物料选项（不区分生豆/耗材）
  getMaterials: (): Promise<Option[]> =>
    api.get('/bom/materials').then(res => res.data),

  // 保存 BOM 出品率
  saveBom: (data: SaveBomRequest): Promise<void> =>
    api.post('/bom/save', data),

  // 保存 BOM 物料项
  saveBomItem: (data: SaveBomItemRequest): Promise<void> =>
    api.post('/bom/item/save', data),

  // 删除 BOM 物料项
  deleteBomItem: (data: DeleteBomItemRequest): Promise<void> =>
    api.post('/bom/item/delete', data),

  // DEV-043: 规格 -> 袋子物料映射
  getBagSpecMappings: (): Promise<BagSpecMapping[]> =>
    api.get('/bom/bag-spec-mappings').then(res => res.data),

  saveBagSpecMapping: (data: SaveBagSpecMappingRequest): Promise<void> =>
    api.post('/bom/bag-spec-mappings/save', data),

  deleteBagSpecMapping: (data: DeleteBagSpecMappingRequest): Promise<void> =>
    api.post('/bom/bag-spec-mappings/delete', data),
}
