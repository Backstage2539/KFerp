import { useMemo, useState } from 'react'
import { useBomList, useBomDetail, useMaterials, useSaveBom, useSaveBomItem, useDeleteBomItem, useBagSpecMappings, useSaveBagSpecMapping, useDeleteBagSpecMapping } from './hooks'
import type { BomListItem, BomItemRow } from './types'

const BOM_REACT_URL = '/bom-react'

const styles = {
  page: { display: 'flex', minHeight: '100vh', fontFamily: 'system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial' } as const,
  sidebar: { width: 220, padding: '14px 12px', borderRight: '1px solid #eee', background: '#fafafa', boxSizing: 'border-box', position: 'sticky', top: 0, height: '100vh', overflow: 'auto' } as const,
  brand: { fontWeight: 800, marginBottom: 10 } as const,
  section: { marginTop: 14, marginBottom: 6, fontSize: 12, color: '#666' } as const,
  link: { display: 'block', padding: '8px 10px', borderRadius: 8, color: '#111', textDecoration: 'none', fontSize: 14 } as const,
  linkActive: { display: 'block', padding: '8px 10px', borderRadius: 8, background: '#111', color: '#fff', textDecoration: 'none', fontSize: 14 } as const,
  main: { flex: 1, padding: 16, boxSizing: 'border-box', background: '#f7f7f8' } as const,
  mainEmbedded: { flex: 1, padding: 16, boxSizing: 'border-box', background: '#fff' } as const,
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

function normalizeKeyword(v: string) {
  return v.trim().toLowerCase().replace(/\s+/g, '')
}

function MaterialAutocomplete({
  materials,
  value,
  onChange,
  placeholder,
  emptyLabel,
}: {
  materials?: { id: number; name: string }[]
  value: string
  onChange: (value: string) => void
  placeholder: string
  emptyLabel: string
}) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)

  const selected = useMemo(
    () => materials?.find((m) => String(m.id) === value) ?? null,
    [materials, value],
  )

  const filtered = useMemo(() => {
    const rows = materials ?? []
    const kw = normalizeKeyword(query)
    if (!kw) return rows.slice(0, 30)
    return rows.filter((m) => normalizeKeyword(m.name).includes(kw)).slice(0, 30)
  }, [materials, query])

  return (
    <div style={{ position: 'relative', flex: 1 }}>
      <input
        style={{ ...styles.input, width: '100%' }}
        value={open ? query : (selected?.name ?? '')}
        placeholder={placeholder}
        onFocus={() => {
          setQuery(selected?.name ?? '')
          setOpen(true)
        }}
        onBlur={() => {
          setTimeout(() => setOpen(false), 120)
        }}
        onChange={(e) => {
          onChange('')
          setQuery(e.target.value)
          setOpen(true)
        }}
      />
      {value && (
        <button
          type="button"
          aria-label="清空物料"
          onClick={() => {
            onChange('')
            setQuery('')
            setOpen(false)
          }}
          style={{
            position: 'absolute',
            right: 10,
            top: 9,
            border: 0,
            background: 'transparent',
            color: '#666',
            cursor: 'pointer',
            fontSize: 16,
          }}
        >
          ×
        </button>
      )}
      {open && (
        <div
          style={{
            position: 'absolute',
            left: 0,
            right: 0,
            top: 'calc(100% + 6px)',
            background: '#fff',
            border: '1px solid #ddd',
            borderRadius: 8,
            boxShadow: '0 6px 18px rgba(0,0,0,.08)',
            maxHeight: 260,
            overflowY: 'auto',
            zIndex: 20,
          }}
        >
          {filtered.length === 0 ? (
            <div style={{ padding: '10px 12px', color: '#666', fontSize: 13 }}>{emptyLabel}</div>
          ) : (
            filtered.map((m) => (
              <button
                key={m.id}
                type="button"
                onMouseDown={(e) => {
                  e.preventDefault()
                  onChange(String(m.id))
                  setQuery(m.name)
                  setOpen(false)
                }}
                style={{
                  display: 'block',
                  width: '100%',
                  textAlign: 'left',
                  padding: '10px 12px',
                  border: 0,
                  borderBottom: '1px solid #f1f1f1',
                  background: '#fff',
                  cursor: 'pointer',
                  fontSize: 14,
                }}
              >
                {m.name}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  )
}

type MenuGroup = { title: string; items: { label: string; href: string; active?: boolean }[] }

const MENU_GROUPS: MenuGroup[] = [
  {
    title: '订单',
    items: [
      { label: '录单', href: '/order' },
      { label: '订单列表', href: '/orders' },
    ],
  },
  {
    title: '生产流程',
    items: [
      { label: '生产计划/开始生产', href: '/produce/unproduced' },
      { label: '生产中', href: '/produce/running' },
      { label: '生产日志', href: '/produce/logs' },
    ],
  },
  {
    title: '物料管理',
    items: [
      { label: '物料档案/库存', href: '/materials' },
      { label: 'BOM配方维护', href: BOM_REACT_URL, active: true },
    ],
  },
  {
    title: '档案',
    items: [
      { label: '客户档案', href: '/customers' },
      { label: '商品档案', href: '/products' },
      { label: '部门维护', href: '/company/departments' },
      { label: '员工维护', href: '/company/employees' },
      { label: '成品库存', href: '/products/inventory' },
      { label: '报价导出', href: '/products/print' },
    ],
  },
  {
    title: '设置',
    items: [{ label: '设备产能配置', href: '/produce/machines' }],
  },
  {
    title: '日志',
    items: [{ label: '操作日志', href: '/audit' }],
  },
  {
    title: '需求管理',
    items: [
      { label: '产品需求表', href: '/req/product' },
      { label: '开发需求表', href: '/req/dev' },
      { label: '单元测试表', href: '/req/unit' },
      { label: 'API 测试表', href: '/req/api' },
      { label: '需求审核表', href: '/req/review' },
    ],
  },
]

function Sidebar() {
  return (
    <aside style={styles.sidebar}>
      <div style={styles.brand}>ERP</div>
      {MENU_GROUPS.map((g) => (
        <div key={g.title}>
          <div style={styles.section}>{g.title}</div>
          {g.items.map((it) => (
            <a key={it.href} style={it.active ? styles.linkActive : styles.link} href={it.href}>
              {it.label}
            </a>
          ))}
        </div>
      ))}
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

  const [newMaterialId, setNewMaterialId] = useState('')
  const [newRatio, setNewRatio] = useState('')
  const [err, setErr] = useState('')

  const syncYield = async () => {
    setErr('')
    await saveBom.mutateAsync({ product_id: productId })
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
        <span>烘焙度：<b>{bomDetail.roast_level || '-'}</b></span>
        <span>出品率：<b>{(bomDetail.yield_rate * 100).toFixed(2)}%</b></span>
        <button style={styles.btn} onClick={syncYield} disabled={saveBom.isPending}>按烘焙度同步</button>
      </div>
      <div style={{ marginBottom: 10, fontSize: 12, color: '#666' }}>浅烘 82%，中烘 81.5%，中深烘 81%，深烘 80%。</div>

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
        <MaterialAutocomplete
          materials={materials}
          value={newMaterialId}
          onChange={setNewMaterialId}
          placeholder="搜索生豆/耗材物料"
          emptyLabel="没有匹配的物料"
        />
        <input style={{ ...styles.input, width: 120 }} type="number" step="0.01" min="0.01" max="100" placeholder="配比%" value={newRatio} onChange={(e) => setNewRatio(e.target.value)} />
        <button style={styles.btn} onClick={addItem} disabled={!newMaterialId || !newRatio || saveBomItem.isPending}>添加</button>
      </div>
      <div style={{ marginTop: 6, fontSize: 12, color: '#666' }}>当前总配比：{bomDetail.total_ratio.toFixed(2)}%</div>
    </div>
  )
}

function BagSpecMappingEditor() {
  const { data: mappings } = useBagSpecMappings()
  const { data: materials } = useMaterials()
  const save = useSaveBagSpecMapping()
  const del = useDeleteBagSpecMapping()

  const [specG, setSpecG] = useState('')
  const [materialId, setMaterialId] = useState('')
  const [err, setErr] = useState('')

  const submit = async () => {
    const spec = parseInt(specG)
    const mid = parseInt(materialId)
    if (!spec || spec <= 0) return setErr('spec_g 必须是正整数')
    if (!mid || mid <= 0) return setErr('请选择袋子物料')
    setErr('')
    await save.mutateAsync({ spec_g: spec, material_id: mid })
    setSpecG('')
    setMaterialId('')
  }

  const onDelete = async (spec: number) => {
    if (!confirm(`确定删除 ${spec}g 的映射？`)) return
    await del.mutateAsync({ spec_g: spec })
  }

  return (
    <div style={styles.card}>
      <div style={styles.cardHead}>DEV-043：包材规格映射维护（spec_g → 袋子物料）</div>
      <div style={styles.cardBody}>
        {err && <div style={styles.alertErr}>{err}</div>}
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 10 }}>
          <input style={{ ...styles.input, width: 160 }} type="number" min="1" placeholder="规格(g)，如 454" value={specG} onChange={(e) => setSpecG(e.target.value)} />
          <MaterialAutocomplete
            materials={materials}
            value={materialId}
            onChange={setMaterialId}
            placeholder="搜索袋子物料"
            emptyLabel="没有匹配的袋子物料"
          />
          <button style={styles.btn} onClick={submit} disabled={!specG || !materialId || save.isPending}>保存映射</button>
        </div>

        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>规格(g)</th>
              <th style={styles.th}>袋子物料</th>
              <th style={{ ...styles.th, textAlign: 'center' }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {mappings?.length ? mappings.map((r) => (
              <tr key={r.spec_g}>
                <td style={styles.td}>{r.spec_g}</td>
                <td style={styles.td}>{r.material_name}</td>
                <td style={{ ...styles.td, textAlign: 'center' }}>
                  <button style={styles.btnGhost} onClick={() => onDelete(r.spec_g)}>删除</button>
                </td>
              </tr>
            )) : <tr><td style={styles.td} colSpan={3}>暂无映射</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default function BomManager() {
  const { data: bomList, isLoading: listLoading } = useBomList()
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const isEmbeddedInShell = new URLSearchParams(window.location.search).get('embed') === '1'

  return (
    <div style={styles.page}>
      {!isEmbeddedInShell && <Sidebar />}
      <main style={isEmbeddedInShell ? styles.mainEmbedded : styles.main}>
        <h2 style={{ margin: '0 0 12px 0' }}>BOM配方维护</h2>
        <BagSpecMappingEditor />
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
                    <th style={styles.th}>烘焙度</th>
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
                        <td style={styles.td}>{row.roast_level || '-'}</td>
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
                          <td style={styles.td} colSpan={6}>
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
