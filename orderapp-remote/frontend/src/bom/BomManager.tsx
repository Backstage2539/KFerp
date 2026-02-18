import { useState } from 'react'
import { useBomList, useBomDetail, useMaterials, useSaveBom, useSaveBomItem, useDeleteBomItem } from './hooks'
import type { BomListItem, BomItemRow } from './types'

const styles = {
  page: { display: 'flex', minHeight: '100vh', fontFamily: 'system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial' } as const,
  sidebar: { width: 220, padding: '14px 12px', borderRight: '1px solid #eee', background: '#fafafa', boxSizing: 'border-box' } as const,
  brand: { fontWeight: 800, marginBottom: 10 } as const,
  section: { marginTop: 14, marginBottom: 6, fontSize: 12, color: '#666' } as const,
  link: { display: 'block', padding: '8px 10px', borderRadius: 8, color: '#111', textDecoration: 'none', fontSize: 14 } as const,
  linkActive: { display: 'block', padding: '8px 10px', borderRadius: 8, background: '#111', color: '#fff', textDecoration: 'none', fontSize: 14 } as const,
  main: { flex: 1, padding: 16, boxSizing: 'border-box', background: '#f7f7f8' } as const,
  card: { background: '#fff', border: '1px solid #eee', borderRadius: 10, marginBottom: 12, overflow: 'hidden' } as const,
  cardHead: { padding: '10px 12px', borderBottom: '1px solid #eee', background: '#fafafa', fontSize: 13, color: '#555' } as const,
  cardBody: { padding: 12 } as const,
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 14 } as const,
  th: { textAlign: 'left', borderBottom: '1px solid #eee', padding: '10px 8px', background: '#fafafa' } as const,
  td: { borderBottom: '1px solid #f1f1f1', padding: '10px 8px', verticalAlign: 'top' } as const,
  btn: { padding: '8px 12px', borderRadius: 8, border: '1px solid #111', background: '#111', color: '#fff', cursor: 'pointer' } as const,
  btnGhost: { padding: '6px 10px', borderRadius: 8, border: '1px solid #999', background: '#fff', color: '#111', cursor: 'pointer' } as const,
  input: { padding: '8px 10px', borderRadius: 8, border: '1px solid #ccc', fontSize: 14 } as const,
  alertErr: { background: '#ffecec', border: '1px solid #ffb9b9', color: '#9a1a1a', padding: 10, borderRadius: 8, marginBottom: 10 } as const,
  alertOk: { background: '#ecffef', border: '1px solid #b9f0c0', color: '#156b26', padding: 10, borderRadius: 8, marginBottom: 10 } as const,
}

function Sidebar() {
  return (
    <aside style={styles.sidebar}>
      <div style={styles.brand}>ERP</div>
      <div style={styles.section}>订单</div>
      <a style={styles.link} href="/order">录单</a>
      <a style={styles.link} href="/orders">订单列表</a>
      <div style={styles.section}>生产流程</div>
      <a style={styles.link} href="/orders?preset=unprod">未生产订单</a>
      <a style={styles.link} href="/produce/unproduced">未生产需求汇总</a>
      <a style={styles.link} href="/produce/plan">生产计划（缺口&gt;0）</a>
      <div style={styles.section}>物料管理</div>
      <a style={styles.link} href="/materials">物料档案/库存</a>
      <a style={styles.linkActive} href="/bom-react">BOM配方维护</a>
      <a style={styles.link} href="/produce/allocations">扣减记录（批次）</a>
    </aside>
  )
}

function InlineEditor({
  productId,
  productName,
  onClose,
}: {
  productId: number
  productName: string
  onClose: () => void
}) {
  const { data: bomDetail, isLoading } = useBomDetail(productId)
  const { data: materials } = useMaterials()
  const saveBom = useSaveBom()
  const saveBomItem = useSaveBomItem()
  const deleteBomItem = useDeleteBomItem()

  const [yieldRate, setYieldRate] = useState('')
  const [newMaterialId, setNewMaterialId] = useState('')
  const [newRatio, setNewRatio] = useState('')
  const [err, setErr] = useState('')

  const saveYield = async () => {
    const rate = parseFloat(yieldRate)
    if (isNaN(rate) || rate <= 0 || rate > 1) return setErr('出品率必须在 (0,1]')
    setErr('')
    await saveBom.mutateAsync({ product_id: productId, yield_rate: rate })
    setYieldRate('')
  }

  const addItem = async () => {
    const mid = parseInt(newMaterialId)
    const ratio = parseFloat(newRatio)
    if (!mid || isNaN(ratio) || ratio <= 0 || ratio > 100) return setErr('配比必须在 (0,100]')
    const total = bomDetail?.total_ratio || 0
    if (total + ratio > 100.0001) return setErr(`总配比超过100%（当前${total.toFixed(2)}%）`)
    setErr('')
    await saveBomItem.mutateAsync({ product_id: productId, material_id: mid, ratio_pct: ratio })
    setNewMaterialId('')
    setNewRatio('')
  }

  const delItem = async (id: number) => {
    if (!confirm('确定删除该物料？')) return
    await deleteBomItem.mutateAsync({ product_id: productId, id })
  }

  if (isLoading || !bomDetail) return <div style={{ padding: 8 }}>加载中...</div>

  return (
    <div style={{ background: '#fbfbfb', border: '1px solid #eee', borderRadius: 8, padding: 10 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
        <b>{productName}（列表内维护）</b>
        <button style={styles.btnGhost} onClick={onClose}>收起</button>
      </div>
      {err && <div style={{ ...styles.alertErr, marginBottom: 8 }}>{err}</div>}

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 10 }}>
        <span>出品率：<b>{(bomDetail.yield_rate * 100).toFixed(2)}%</b></span>
        <input style={styles.input} type="number" step="0.0001" min="0.0001" max="1" placeholder="新出品率(0-1)" value={yieldRate} onChange={(e) => setYieldRate(e.target.value)} />
        <button style={styles.btn} onClick={saveYield} disabled={!yieldRate || saveBom.isPending}>保存</button>
      </div>

      <table style={styles.table}>
        <thead>
          <tr>
            <th style={styles.th}>物料</th>
            <th style={{ ...styles.th, textAlign: 'right' }}>配比%</th>
            <th style={{ ...styles.th, textAlign: 'center' }}>操作</th>
          </tr>
        </thead>
        <tbody>
          {bomDetail.items.length === 0 ? (
            <tr><td style={styles.td} colSpan={3}>暂无物料</td></tr>
          ) : bomDetail.items.map((item: BomItemRow) => (
            <tr key={item.id}>
              <td style={styles.td}>{item.material_name}</td>
              <td style={{ ...styles.td, textAlign: 'right' }}>{item.ratio_pct.toFixed(2)}%</td>
              <td style={{ ...styles.td, textAlign: 'center' }}><button style={styles.btnGhost} onClick={() => delItem(item.id)}>删除</button></td>
            </tr>
          ))}
        </tbody>
      </table>

      <div style={{ marginTop: 10, display: 'flex', gap: 8, alignItems: 'center' }}>
        <select style={{ ...styles.input, flex: 1 }} value={newMaterialId} onChange={(e) => setNewMaterialId(e.target.value)}>
          <option value="">选择物料（生豆/耗材统一）</option>
          {materials?.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
        </select>
        <input style={{ ...styles.input, width: 120 }} type="number" step="0.01" min="0.01" max="100" placeholder="配比%" value={newRatio} onChange={(e) => setNewRatio(e.target.value)} />
        <button style={styles.btn} onClick={addItem} disabled={!newMaterialId || !newRatio || saveBomItem.isPending}>添加</button>
      </div>
      <div style={{ marginTop: 6, fontSize: 12, color: '#666' }}>当前总配比：{bomDetail.total_ratio.toFixed(2)}%</div>
    </div>
  )
}

export default function BomManager() {
  const { data: bomList, isLoading: listLoading } = useBomList()
  const [expandedId, setExpandedId] = useState<number | null>(null)

  return (
    <div style={styles.page}>
      <Sidebar />
      <main style={styles.main}>
        <h2 style={{ margin: '0 0 12px 0' }}>BOM配方维护</h2>
        <div style={styles.card}>
          <div style={styles.cardHead}>列表直接维护（无需跳详情）</div>
          <div style={styles.cardBody}>
            {listLoading ? (
              <div>加载中...</div>
            ) : (
              <table style={styles.table}>
                <thead>
                  <tr>
                    <th style={styles.th}>产品名称</th>
                    <th style={{ ...styles.th, textAlign: 'right' }}>出品率</th>
                    <th style={{ ...styles.th, textAlign: 'right' }}>物料数</th>
                    <th style={{ ...styles.th, textAlign: 'right' }}>更新时间</th>
                    <th style={{ ...styles.th, textAlign: 'center' }}>维护</th>
                  </tr>
                </thead>
                <tbody>
                  {bomList?.map((row: BomListItem) => (
                    <>
                      <tr key={row.product_id}>
                        <td style={styles.td}>{row.product}</td>
                        <td style={{ ...styles.td, textAlign: 'right' }}>{(row.yield_rate * 100).toFixed(2)}%</td>
                        <td style={{ ...styles.td, textAlign: 'right' }}>{row.item_count || 0}</td>
                        <td style={{ ...styles.td, textAlign: 'right', color: '#666' }}>{row.updated_at}</td>
                        <td style={{ ...styles.td, textAlign: 'center' }}>
                          <button
                            style={styles.btnGhost}
                            onClick={() => setExpandedId(expandedId === row.product_id ? null : row.product_id)}
                          >
                            {expandedId === row.product_id ? '收起' : '在列表维护'}
                          </button>
                        </td>
                      </tr>
                      {expandedId === row.product_id && (
                        <tr>
                          <td style={styles.td} colSpan={5}>
                            <InlineEditor productId={row.product_id} productName={row.product} onClose={() => setExpandedId(null)} />
                          </td>
                        </tr>
                      )}
                    </>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </main>
    </div>
  )
}
