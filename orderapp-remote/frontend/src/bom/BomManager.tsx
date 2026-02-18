import { useState } from 'react'
import { useBomList, useBomDetail, useMaterials, useSaveBom, useSaveBomItem, useDeleteBomItem } from './hooks'
import type { BomListItem, BomItemRow } from './types'

function Sidebar() {
  const linkClass = 'block px-3 py-2 rounded hover:bg-gray-100 text-sm text-gray-800'
  return (
    <aside className="w-56 border-r bg-gray-50 min-h-screen p-3 sticky top-0">
      <div className="font-bold mb-2">ERP</div>
      <div className="text-xs text-gray-500 mt-3 mb-1">订单</div>
      <a className={linkClass} href="/order">录单</a>
      <a className={linkClass} href="/orders">订单列表</a>

      <div className="text-xs text-gray-500 mt-3 mb-1">生产流程</div>
      <a className={linkClass} href="/orders?preset=unprod">未生产订单</a>
      <a className={linkClass} href="/produce/unproduced">未生产需求汇总</a>
      <a className={linkClass} href="/produce/plan">生产计划（缺口&gt;0）</a>

      <div className="text-xs text-gray-500 mt-3 mb-1">物料管理</div>
      <a className={linkClass} href="/materials">物料档案/库存</a>
      <a className="block px-3 py-2 rounded bg-blue-600 text-white text-sm" href="/bom-react">BOM配方维护</a>
      <a className={linkClass} href="/produce/allocations">扣减记录（批次）</a>

      <div className="text-xs text-gray-500 mt-3 mb-1">档案</div>
      <a className={linkClass} href="/customers">客户档案</a>
      <a className={linkClass} href="/products">商品档案</a>
      <a className={linkClass} href="/products/inventory">成品库存</a>
    </aside>
  )
}

export default function BomManager() {
  const [selectedProductId, setSelectedProductId] = useState<number | null>(null)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [yieldRate, setYieldRate] = useState('')
  const [newMaterialId, setNewMaterialId] = useState('')
  const [newRatio, setNewRatio] = useState('')

  const { data: bomList, isLoading: listLoading } = useBomList()
  const { data: bomDetail, isLoading: detailLoading } = useBomDetail(selectedProductId)
  const { data: materials } = useMaterials()

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
      await saveBomItem.mutateAsync({ product_id: selectedProductId, material_id: mid, ratio_pct: ratio })
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

  return (
    <div className="min-h-screen flex bg-gray-50">
      <Sidebar />
      <main className="flex-1 p-6">
        <h1 className="text-2xl font-bold mb-4">BOM配方维护</h1>

        {error && <div className="bg-red-100 border border-red-300 text-red-700 px-4 py-2 rounded mb-3">{error}</div>}
        {success && <div className="bg-green-100 border border-green-300 text-green-700 px-4 py-2 rounded mb-3">{success}</div>}

        {!selectedProductId ? (
          <div className="bg-white rounded shadow">
            <div className="px-4 py-3 border-b bg-gray-50 text-sm text-gray-700">列表维护 BOM（点击产品进入详情维护）</div>
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
                    <tr key={row.product_id} className="border-b hover:bg-blue-50 cursor-pointer" onClick={() => handleSelectProduct(row.product_id)}>
                      <td className="py-3 px-4 font-medium">{row.product}</td>
                      <td className="text-right py-3 px-4">{(row.yield_rate * 100).toFixed(2)}%</td>
                      <td className="text-right py-3 px-4">{row.item_count || 0}</td>
                      <td className="text-right py-3 px-4 text-gray-500">{row.updated_at}</td>
                      <td className="text-center py-3 px-4 text-blue-600">查看详情</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        ) : (
          <div className="space-y-4">
            <button onClick={() => setSelectedProductId(null)} className="text-blue-600 text-sm">← 返回列表</button>

            {detailLoading ? (
              <div className="text-gray-500">加载中...</div>
            ) : bomDetail ? (
              <>
                <div className="bg-white rounded shadow p-4">
                  <div className="font-medium mb-2">{bomDetail.product_name}（单 batch 物料）</div>
                  <div className="flex items-center gap-3">
                    <div className="text-xl font-bold text-blue-600">{(bomDetail.yield_rate * 100).toFixed(2)}%</div>
                    <input type="number" step="0.0001" min="0.0001" max="1" placeholder="新出品率(0-1)" className="border rounded px-3 py-2 w-44" value={yieldRate} onChange={(e) => setYieldRate(e.target.value)} />
                    <button onClick={handleSaveYieldRate} disabled={saveBom.isPending || !yieldRate} className="bg-blue-600 text-white px-3 py-2 rounded disabled:opacity-50">保存出品率</button>
                  </div>
                </div>

                <div className="bg-white rounded shadow p-4">
                  <div className="flex justify-between items-center mb-3">
                    <div className="font-medium">物料清单（生豆/耗材统一维度）</div>
                    <div className="text-sm">总配比: <b>{bomDetail.total_ratio.toFixed(2)}%</b></div>
                  </div>

                  <table className="w-full text-sm mb-4">
                    <thead>
                      <tr className="border-b bg-gray-50">
                        <th className="text-left py-2 px-3">物料</th>
                        <th className="text-right py-2 px-3">配比%</th>
                        <th className="text-center py-2 px-3">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {bomDetail.items.length === 0 ? (
                        <tr><td className="py-6 text-center text-gray-500" colSpan={3}>暂无物料</td></tr>
                      ) : bomDetail.items.map((item: BomItemRow) => (
                        <tr key={item.id} className="border-b">
                          <td className="py-2 px-3">{item.material_name}</td>
                          <td className="text-right py-2 px-3">{item.ratio_pct.toFixed(2)}%</td>
                          <td className="text-center py-2 px-3"><button className="text-red-600" onClick={() => handleDeleteItem(item.id)}>删除</button></td>
                        </tr>
                      ))}
                    </tbody>
                  </table>

                  <div className="border-t pt-3 flex gap-2">
                    <select value={newMaterialId} onChange={(e) => setNewMaterialId(e.target.value)} className="border rounded px-3 py-2 flex-1">
                      <option value="">选择物料</option>
                      {materials?.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
                    </select>
                    <input type="number" step="0.01" min="0.01" max="100" placeholder="配比%" className="border rounded px-3 py-2 w-32" value={newRatio} onChange={(e) => setNewRatio(e.target.value)} />
                    <button onClick={handleAddItem} disabled={saveBomItem.isPending || !newMaterialId || !newRatio} className="bg-green-600 text-white px-4 py-2 rounded disabled:opacity-50">添加</button>
                  </div>
                </div>
              </>
            ) : null}
          </div>
        )}
      </main>
    </div>
  )
}
