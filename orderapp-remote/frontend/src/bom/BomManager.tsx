import { useState } from 'react'
import { useBomList, useBomDetail, useProducts, useBeanMaterials, useSaveBom, useSaveBomItem, useDeleteBomItem } from './hooks'
import type { BomRow, BomItemRow } from './types'

export default function BomManager() {
  const [selectedProductId, setSelectedProductId] = useState<number | null>(null)
  const [error, setError] = useState<string>('')
  const [success, setSuccess] = useState<boolean>(false)

  // Form states
  const [yieldRate, setYieldRate] = useState<string>('')
  const [newMaterialId, setNewMaterialId] = useState<string>('')
  const [newRatio, setNewRatio] = useState<string>('')

  // Queries
  const { data: bomList, isLoading: listLoading } = useBomList()
  const { data: bomDetail, isLoading: detailLoading } = useBomDetail(selectedProductId)
  useProducts() // prefetch products for potential use
  const { data: materials } = useBeanMaterials()

  // Mutations
  const saveBom = useSaveBom()
  const saveBomItem = useSaveBomItem()
  const deleteBomItem = useDeleteBomItem()

  const handleSelectProduct = (id: number) => {
    setSelectedProductId(id)
    setError('')
    setSuccess(false)
    setYieldRate('')
    setNewMaterialId('')
    setNewRatio('')
  }

  const handleSaveBom = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedProductId) return

    const rate = parseFloat(yieldRate)
    if (isNaN(rate) || rate <= 0 || rate > 1) {
      setError('出品率必须在 (0, 1] 之间')
      return
    }

    try {
      await saveBom.mutateAsync({ product_id: selectedProductId, yield_rate: rate })
      setSuccess(true)
      setError('')
      setTimeout(() => setSuccess(false), 2000)
    } catch (err: any) {
      setError(err.response?.data?.error || '保存失败')
    }
  }

  const handleAddItem = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedProductId) return

    const mid = parseInt(newMaterialId)
    const ratio = parseFloat(newRatio)

    if (!mid || isNaN(ratio) || ratio <= 0 || ratio > 100) {
      setError('配比必须在 (0, 100] 之间')
      return
    }

    // Check total ratio
    const currentTotal = bomDetail?.total_ratio || 0
    if (currentTotal + ratio > 100.0001) {
      setError(`配比总和不能超过 100% (当前 ${currentTotal.toFixed(2)}%)`)
      return
    }

    try {
      await saveBomItem.mutateAsync({
        product_id: selectedProductId,
        material_id: mid,
        ratio_pct: ratio,
      })
      setNewMaterialId('')
      setNewRatio('')
      setError('')
    } catch (err: any) {
      setError(err.response?.data?.error || '添加失败')
    }
  }

  const handleDeleteItem = async (itemId: number) => {
    if (!selectedProductId) return
    if (!confirm('确定删除该物料配比？')) return

    try {
      await deleteBomItem.mutateAsync({ product_id: selectedProductId, id: itemId })
    } catch (err: any) {
      setError(err.response?.data?.error || '删除失败')
    }
  }

  return (
    <div className="container mx-auto px-4 py-6">
      <h1 className="text-2xl font-bold mb-6">BOM 配方维护</h1>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {success && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
          保存成功！
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left: BOM List */}
        <div className="bg-white rounded-lg shadow p-4">
          <h2 className="text-lg font-semibold mb-4">产品 BOM 列表</h2>
          {listLoading ? (
            <div className="text-gray-500">加载中...</div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2">产品</th>
                  <th className="text-right py-2">出品率</th>
                  <th className="text-right py-2">更新于</th>
                </tr>
              </thead>
              <tbody>
                {bomList?.map((row: BomRow) => (
                  <tr
                    key={row.product_id}
                    className={`border-b cursor-pointer hover:bg-gray-50 ${
                      selectedProductId === row.product_id ? 'bg-blue-50' : ''
                    }`}
                    onClick={() => handleSelectProduct(row.product_id)}
                  >
                    <td className="py-2">{row.product}</td>
                    <td className="text-right py-2">{(row.yield_rate * 100).toFixed(2)}%</td>
                    <td className="text-right py-2 text-gray-500">{row.updated_at}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Right: BOM Detail */}
        <div className="bg-white rounded-lg shadow p-4">
          {selectedProductId ? (
            detailLoading ? (
              <div className="text-gray-500">加载中...</div>
            ) : bomDetail ? (
              <div>
                <h2 className="text-lg font-semibold mb-4">{bomDetail.product_name}</h2>

                {/* Yield Rate Form */}
                <form onSubmit={handleSaveBom} className="mb-6 p-4 bg-gray-50 rounded">
                  <h3 className="font-medium mb-3">出品率设置</h3>
                  <div className="flex gap-2">
                    <input
                      type="number"
                      step="0.0001"
                      min="0.0001"
                      max="1"
                      placeholder="出品率 (0-1)"
                      className="flex-1 border rounded px-3 py-2"
                      value={yieldRate}
                      onChange={(e) => setYieldRate(e.target.value)}
                      defaultValue={bomDetail.yield_rate}
                    />
                    <button
                      type="submit"
                      disabled={saveBom.isPending}
                      className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 disabled:opacity-50"
                    >
                      {saveBom.isPending ? '保存中...' : '保存出品率'}
                    </button>
                  </div>
                  <p className="text-xs text-gray-500 mt-1">当前: {(bomDetail.yield_rate * 100).toFixed(2)}%</p>
                </form>

                {/* Items List */}
                <div className="mb-4">
                  <h3 className="font-medium mb-3">物料配比</h3>
                  <div className="text-sm text-gray-600 mb-2">
                    当前配比总和: <span className={bomDetail.total_ratio > 100 ? 'text-red-600 font-bold' : 'font-bold'}>
                      {bomDetail.total_ratio.toFixed(2)}%
                    </span>
                  </div>

                  {bomDetail.items.length > 0 ? (
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b">
                          <th className="text-left py-2">物料</th>
                          <th className="text-right py-2">配比%</th>
                          <th className="text-right py-2">操作</th>
                        </tr>
                      </thead>
                      <tbody>
                        {bomDetail.items.map((item: BomItemRow) => (
                          <tr key={item.id} className="border-b">
                            <td className="py-2">{item.material_name}</td>
                            <td className="text-right py-2">{item.ratio_pct.toFixed(2)}%</td>
                            <td className="text-right py-2">
                              <button
                                onClick={() => handleDeleteItem(item.id)}
                                disabled={deleteBomItem.isPending}
                                className="text-red-600 hover:text-red-800 text-xs"
                              >
                                删除
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  ) : (
                    <div className="text-gray-500 text-sm py-4">暂无物料配比</div>
                  )}
                </div>

                {/* Add Item Form */}
                <form onSubmit={handleAddItem} className="p-4 bg-gray-50 rounded">
                  <h3 className="font-medium mb-3">添加物料配比</h3>
                  <div className="grid grid-cols-2 gap-2 mb-2">
                    <select
                      value={newMaterialId}
                      onChange={(e) => setNewMaterialId(e.target.value)}
                      className="border rounded px-3 py-2"
                      required
                    >
                      <option value="">选择物料</option>
                      {materials?.map((m) => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </select>
                    <input
                      type="number"
                      step="0.01"
                      min="0.01"
                      max="100"
                      placeholder="配比 %"
                      className="border rounded px-3 py-2"
                      value={newRatio}
                      onChange={(e) => setNewRatio(e.target.value)}
                      required
                    />
                  </div>
                  <button
                    type="submit"
                    disabled={saveBomItem.isPending || !newMaterialId || !newRatio}
                    className="w-full bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700 disabled:opacity-50"
                  >
                    {saveBomItem.isPending ? '添加中...' : '添加物料'}
                  </button>
                </form>
              </div>
            ) : null
          ) : (
            <div className="text-gray-500 text-center py-12">请选择左侧产品查看详情</div>
          )}
        </div>
      </div>
    </div>
  )
}
