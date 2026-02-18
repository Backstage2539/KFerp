import { useState } from 'react'
import { useBomList, useBomDetail, useMaterials, useSaveBom, useSaveBomItem, useDeleteBomItem } from './hooks'
import type { BomListItem, BomItemRow } from './types'

export default function BomManager() {
  const [selectedProductId, setSelectedProductId] = useState<number | null>(null)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  // Form states for editing
  const [yieldRate, setYieldRate] = useState('')
  const [newMaterialId, setNewMaterialId] = useState('')
  const [newRatio, setNewRatio] = useState('')

  // Queries
  const { data: bomList, isLoading: listLoading } = useBomList()
  const { data: bomDetail, isLoading: detailLoading } = useBomDetail(selectedProductId)
  const { data: materials } = useMaterials()

  // Mutations
  const saveBom = useSaveBom()
  const saveBomItem = useSaveBomItem()
  const deleteBomItem = useDeleteBomItem()

  const showSuccess = (msg: string) => {
    setSuccess(msg)
    setTimeout(() => setSuccess(''), 2000)
  }

  const handleSelectProduct = (id: number) => {
    setSelectedProductId(id)
    setError('')
    setSuccess('')
    setYieldRate('')
    setNewMaterialId('')
    setNewRatio('')
  }

  const handleBackToList = () => {
    setSelectedProductId(null)
    setError('')
    setSuccess('')
  }

  const handleSaveYieldRate = async () => {
    if (!selectedProductId) return
    const rate = parseFloat(yieldRate)
    if (isNaN(rate) || rate <= 0 || rate > 1) {
      setError('出品率必须在 (0, 1] 之间')
      return
    }

    try {
      await saveBom.mutateAsync({ product_id: selectedProductId, yield_rate: rate })
      showSuccess('出品率保存成功')
      setYieldRate('')
    } catch (err: any) {
      setError(err.response?.data?.error || '保存失败')
    }
  }

  const handleAddItem = async () => {
    if (!selectedProductId) return

    const mid = parseInt(newMaterialId)
    const ratio = parseFloat(newRatio)

    if (!mid || isNaN(ratio) || ratio <= 0 || ratio > 100) {
      setError('配比必须在 (0, 100] 之间')
      return
    }

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
      showSuccess('物料添加成功')
      setNewMaterialId('')
      setNewRatio('')
    } catch (err: any) {
      setError(err.response?.data?.error || '添加失败')
    }
  }

  const handleDeleteItem = async (itemId: number) => {
    if (!selectedProductId) return
    if (!confirm('确定删除该物料配比？')) return

    try {
      await deleteBomItem.mutateAsync({ product_id: selectedProductId, id: itemId })
      showSuccess('物料删除成功')
    } catch (err: any) {
      setError(err.response?.data?.error || '删除失败')
    }
  }

  // Render BOM List View
  if (!selectedProductId) {
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
            {success}
          </div>
        )}

        <div className="bg-white rounded-lg shadow">
          <div className="px-4 py-3 border-b bg-gray-50">
            <span className="font-medium">产品 BOM 列表</span>
            <span className="text-sm text-gray-500 ml-2">（点击产品查看详情）</span>
          </div>
          
          {listLoading ? (
            <div className="p-4 text-gray-500">加载中...</div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-gray-50">
                  <th className="text-left py-3 px-4">产品名称</th>
                  <th className="text-right py-3 px-4">出品率</th>
                  <th className="text-right py-3 px-4">物料数</th>
                  <th className="text-right py-3 px-4">更新时间</th>
                  <th className="text-center py-3 px-4">操作</th>
                </tr>
              </thead>
              <tbody>
                {bomList?.map((row: BomListItem) => (
                  <tr
                    key={row.product_id}
                    className="border-b hover:bg-blue-50 cursor-pointer"
                    onClick={() => handleSelectProduct(row.product_id)}
                  >
                    <td className="py-3 px-4 font-medium">{row.product}</td>
                    <td className="text-right py-3 px-4">{(row.yield_rate * 100).toFixed(2)}%</td>
                    <td className="text-right py-3 px-4">{row.item_count || 0}</td>
                    <td className="text-right py-3 px-4 text-gray-500">{row.updated_at}</td>
                    <td className="text-center py-3 px-4">
                      <button className="text-blue-600 hover:text-blue-800 text-sm">
                        查看详情
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    )
  }

  // Render BOM Detail View
  return (
    <div className="container mx-auto px-4 py-6">
      <div className="flex items-center gap-4 mb-6">
        <button
          onClick={handleBackToList}
          className="text-blue-600 hover:text-blue-800 text-sm"
        >
          ← 返回列表
        </button>
        <h1 className="text-2xl font-bold">
          {detailLoading ? '加载中...' : bomDetail?.product_name || '产品详情'}
        </h1>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}
      {success && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
          {success}
        </div>
      )}

      {detailLoading ? (
        <div className="text-gray-500">加载中...</div>
      ) : bomDetail ? (
        <div className="space-y-6">
          {/* Yield Rate Section */}
          <div className="bg-white rounded-lg shadow p-4">
            <h2 className="font-medium mb-4">出品率设置</h2>
            <div className="flex items-center gap-4">
              <div className="text-2xl font-bold text-blue-600">
                {(bomDetail.yield_rate * 100).toFixed(2)}%
              </div>
              <div className="flex-1 flex gap-2">
                <input
                  type="number"
                  step="0.0001"
                  min="0.0001"
                  max="1"
                  placeholder="新出品率 (0-1)"
                  className="border rounded px-3 py-2 w-40"
                  value={yieldRate}
                  onChange={(e) => setYieldRate(e.target.value)}
                />
                <button
                  onClick={handleSaveYieldRate}
                  disabled={saveBom.isPending || !yieldRate}
                  className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 disabled:opacity-50"
                >
                  {saveBom.isPending ? '保存中...' : '保存出品率'}
                </button>
              </div>
            </div>
          </div>

          {/* BOM Items List */}
          <div className="bg-white rounded-lg shadow p-4">
            <div className="flex justify-between items-center mb-4">
              <h2 className="font-medium">物料配比列表</h2>
              <div className="text-sm text-gray-600">
                配比总和: <span className={bomDetail.total_ratio > 100 ? 'text-red-600 font-bold' : 'font-bold'}>
                  {bomDetail.total_ratio.toFixed(2)}%
                </span>
              </div>
            </div>

            {bomDetail.items.length > 0 ? (
              <table className="w-full text-sm mb-4">
                <thead>
                  <tr className="border-b bg-gray-50">
                    <th className="text-left py-2 px-3">物料名称</th>
                    <th className="text-right py-2 px-3">配比 %</th>
                    <th className="text-center py-2 px-3">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {bomDetail.items.map((item: BomItemRow) => (
                    <tr key={item.id} className="border-b hover:bg-gray-50">
                      <td className="py-2 px-3">{item.material_name}</td>
                      <td className="text-right py-2 px-3">{item.ratio_pct.toFixed(2)}%</td>
                      <td className="text-center py-2 px-3">
                        <button
                          onClick={() => handleDeleteItem(item.id)}
                          disabled={deleteBomItem.isPending}
                          className="text-red-600 hover:text-red-800 text-sm"
                        >
                          删除
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="text-gray-500 py-8 text-center mb-4">暂无物料配比</div>
            )}

            {/* Add Item Form */}
            <div className="border-t pt-4">
              <h3 className="font-medium mb-3">添加物料</h3>
              <div className="flex gap-2">
                <select
                  value={newMaterialId}
                  onChange={(e) => setNewMaterialId(e.target.value)}
                  className="border rounded px-3 py-2 flex-1"
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
                  className="border rounded px-3 py-2 w-32"
                  value={newRatio}
                  onChange={(e) => setNewRatio(e.target.value)}
                />
                <button
                  onClick={handleAddItem}
                  disabled={saveBomItem.isPending || !newMaterialId || !newRatio}
                  className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700 disabled:opacity-50"
                >
                  {saveBomItem.isPending ? '添加中...' : '添加'}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
