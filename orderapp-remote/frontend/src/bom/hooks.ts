import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bomApi } from './api'
import type { SaveBomRequest, SaveBomItemRequest, DeleteBomItemRequest } from './types'

export const useBomList = () =>
  useQuery({
    queryKey: ['bom', 'list'],
    queryFn: bomApi.getBomList,
  })

export const useBomDetail = (productId: number | null) =>
  useQuery({
    queryKey: ['bom', 'detail', productId],
    queryFn: () => bomApi.getBomDetail(productId!),
    enabled: !!productId,
  })

export const useProducts = () =>
  useQuery({
    queryKey: ['bom', 'products'],
    queryFn: bomApi.getProducts,
  })

export const useMaterials = () =>
  useQuery({
    queryKey: ['bom', 'materials'],
    queryFn: bomApi.getMaterials,
  })

export const useSaveBom = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: SaveBomRequest) => bomApi.saveBom(data),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ['bom', 'list'] })
      qc.invalidateQueries({ queryKey: ['bom', 'detail', vars.product_id] })
    },
  })
}

export const useSaveBomItem = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: SaveBomItemRequest) => bomApi.saveBomItem(data),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ['bom', 'detail', vars.product_id] })
      qc.invalidateQueries({ queryKey: ['bom', 'list'] })
    },
  })
}

export const useDeleteBomItem = () => {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: DeleteBomItemRequest) => bomApi.deleteBomItem(data),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ['bom', 'detail', vars.product_id] })
      qc.invalidateQueries({ queryKey: ['bom', 'list'] })
    },
  })
}
