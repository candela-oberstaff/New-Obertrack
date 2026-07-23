import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  Wallet as WalletIcon,
  RefreshCw,
  Clock,
  Receipt,
  CheckCircle2,
  XCircle,
  FileSpreadsheet,
  FileText,
  Search,
  ShieldAlert,
  Calendar,
  Hash,
  CalendarClock,
  TrendingUp,
  TrendingDown,
  HelpCircle,
} from 'lucide-react'
import { Button, Skeleton, Modal } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { walletService, type MyPayment, type PaymentStatus } from '../services/wallet.service'
import styles from './Wallet.module.css'

// Clasifica un pago por tipo a partir de su descripción (heurística).
type PayKind = 'Sueldo' | 'Bono' | 'Reembolso'
const kindOf = (desc: string): PayKind => {
  const d = desc.toLowerCase()
  if (d.includes('bono') || d.includes('bonus')) return 'Bono'
  if (d.includes('reembolso') || d.includes('reintegro')) return 'Reembolso'
  return 'Sueldo'
}
const KIND_COLOR: Record<PayKind, string> = {
  Sueldo: 'var(--primary)',
  Bono: 'var(--accent-alt)',
  Reembolso: '#f59e0b',
}

const PAYMENT_LABEL: Record<PaymentStatus, string> = {
  PENDING: 'Pendiente',
  SUCCESS: 'Cobrado',
  FAILURE: 'Fallido',
  REJECTED: 'Rechazado',
  ERROR: 'Error',
}

const STATUS_TONE: Record<PaymentStatus, 'ok' | 'wait' | 'bad'> = {
  SUCCESS: 'ok',
  PENDING: 'wait',
  FAILURE: 'bad',
  REJECTED: 'bad',
  ERROR: 'bad',
}

const MONTHS_ABBR = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic']

type FilterKey = 'all' | 'SUCCESS' | 'PENDING' | 'bad'

const FILTERS: { key: FilterKey; label: string; match: (s: PaymentStatus) => boolean }[] = [
  { key: 'all', label: 'Todos', match: () => true },
  { key: 'SUCCESS', label: 'Cobrados', match: (s) => s === 'SUCCESS' },
  { key: 'PENDING', label: 'Pendientes', match: (s) => s === 'PENDING' },
  { key: 'bad', label: 'Rechazados', match: (s) => s === 'REJECTED' || s === 'FAILURE' || s === 'ERROR' },
]

const fmtMoney = (n: number, currency?: string) =>
  new Intl.NumberFormat('es', {
    style: 'currency',
    currency: currency && currency.length === 3 ? currency : 'USD',
  }).format(Number.isFinite(n) ? n : 0)

const parseDate = (iso?: string) => {
  if (!iso) return null
  const d = new Date(iso.length === 10 ? `${iso}T00:00:00` : iso)
  return isNaN(d.getTime()) ? null : d
}

const fmtDate = (iso?: string) => {
  const d = parseDate(iso)
  return d ? d.toLocaleDateString('es', { day: '2-digit', month: 'short', year: 'numeric' }) : '—'
}

const escapeHtml = (s: string) =>
  s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!))

export default function Wallet() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const walletQ = useQuery({ queryKey: ['wallet', 'me'], queryFn: walletService.myWallet })
  const [filter, setFilter] = useState<FilterKey>('all')
  const [search, setSearch] = useState('')
  const [detail, setDetail] = useState<MyPayment | null>(null)

  const enabled = walletQ.data?.enabled
  const summary = walletQ.data?.summary
  const currency = summary?.currency
  const payments = summary?.payments ?? []

  // Orden cronológico descendente (los sin fecha, al final).
  const sorted = useMemo(() => {
    return [...payments].sort((a, b) => {
      const da = parseDate(a.date)?.getTime() ?? 0
      const db = parseDate(b.date)?.getTime() ?? 0
      return db - da
    })
  }, [payments])

  const filtered = useMemo(() => {
    const f = FILTERS.find((x) => x.key === filter) ?? FILTERS[0]
    const q = search.trim().toLowerCase()
    return sorted.filter(
      (p) =>
        f.match(p.status) &&
        (!q || p.description.toLowerCase().includes(q) || (p.cause ?? '').toLowerCase().includes(q)),
    )
  }, [sorted, filter, search])

  // Ganancias cobradas por mes (rango continuo, últimos 6 meses con datos).
  const monthly = useMemo(() => {
    const byMonth = new Map<string, number>()
    for (const p of payments) {
      if (p.status !== 'SUCCESS') continue
      const d = parseDate(p.date)
      if (!d) continue
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      byMonth.set(key, (byMonth.get(key) ?? 0) + p.amount)
    }
    const keys = [...byMonth.keys()].sort()
    if (!keys.length) return []
    const [minY, minM] = keys[0].split('-').map(Number)
    const [maxY, maxM] = keys[keys.length - 1].split('-').map(Number)
    const buckets: { key: string; label: string; total: number }[] = []
    let y = minY
    let m = minM
    while (y < maxY || (y === maxY && m <= maxM)) {
      const key = `${y}-${String(m).padStart(2, '0')}`
      buckets.push({ key, label: MONTHS_ABBR[m - 1], total: byMonth.get(key) ?? 0 })
      m++
      if (m > 12) {
        m = 1
        y++
      }
    }
    return buckets.slice(-6)
  }, [payments])

  const rejected = useMemo(() => payments.filter((p) => STATUS_TONE[p.status] === 'bad'), [payments])

  // Próximo pago estimado: el pago PENDING más próximo por fecha.
  const nextPayment = useMemo(() => {
    const pend = payments.filter((p) => p.status === 'PENDING')
    if (!pend.length) return null
    return [...pend].sort(
      (a, b) => (parseDate(a.date)?.getTime() ?? Infinity) - (parseDate(b.date)?.getTime() ?? Infinity),
    )[0]
  }, [payments])

  // Variación del último mes cobrado vs. el anterior (a partir de `monthly`).
  const monthDelta = useMemo(() => {
    if (monthly.length < 2) return null
    const last = monthly[monthly.length - 1].total
    const prev = monthly[monthly.length - 2].total
    if (prev <= 0) return null
    return ((last - prev) / prev) * 100
  }, [monthly])

  // Desglose de lo cobrado por tipo (sueldo / bono / reembolso).
  const breakdown = useMemo(() => {
    const acc: Record<PayKind, number> = { Sueldo: 0, Bono: 0, Reembolso: 0 }
    for (const p of payments) {
      if (p.status !== 'SUCCESS') continue
      acc[kindOf(p.description)] += p.amount
    }
    const total = acc.Sueldo + acc.Bono + acc.Reembolso
    return { acc, total }
  }, [payments])

  const stamp = () => new Date().toISOString().slice(0, 10)

  // Exporta a Excel reutilizando write-excel-file (mismo patrón que ExportUsersModal).
  const exportExcel = async () => {
    const { default: writeXlsxFile } = await import('write-excel-file/browser')
    const header = ['Fecha', 'Concepto', 'Monto', 'Moneda', 'Estado', 'Motivo'].map((h) => ({
      value: h,
      type: String,
      fontWeight: 'bold' as const,
      backgroundColor: '#EDE9FE',
      color: '#5B21B6',
    }))
    const body = sorted.map((p) => [
      { value: p.date ?? '', type: String },
      { value: p.description, type: String },
      { value: p.amount, type: Number, format: '#,##0.00' },
      { value: currency ?? 'USD', type: String },
      { value: PAYMENT_LABEL[p.status], type: String },
      { value: p.cause ?? '', type: String },
    ])
    const { toFile } = await writeXlsxFile([header, ...body], {
      sheet: 'Mis pagos',
      stickyRowsCount: 1,
      columns: [{ width: 14 }, { width: 34 }, { width: 12 }, { width: 10 }, { width: 14 }, { width: 34 }],
    })
    await toFile(`mis-pagos-${stamp()}.xlsx`)
  }

  // Genera un PDF vía impresión del navegador: abre un recibo limpio y dispara
  // el diálogo de impresión (el usuario elige "Guardar como PDF"). Sin dependencias.
  // Si se pasa `only`, genera el recibo de UN pago; si no, todo el historial.
  const exportPdf = (only?: MyPayment) => {
    const list = only ? [only] : sorted
    const isReceipt = !!only
    const win = window.open('', '_blank', 'width=820,height=920')
    if (!win) return
    const rows = list
      .map(
        (p) => `<tr>
          <td>${fmtDate(p.date)}</td>
          <td>${escapeHtml(p.description)}${p.cause ? `<div class="cause">${escapeHtml(p.cause)}</div>` : ''}</td>
          <td class="r">${fmtMoney(p.amount, currency)}</td>
          <td>${PAYMENT_LABEL[p.status]}</td>
        </tr>`,
      )
      .join('')
    win.document.write(`<!doctype html><html lang="es"><head><meta charset="utf-8">
      <title>${isReceipt ? 'Recibo de pago' : 'Mis pagos'} - Obertrack</title>
      <style>
        * { box-sizing: border-box; }
        body { font-family: 'Segoe UI', Arial, sans-serif; color: #0f172a; margin: 40px; }
        .head { display:flex; justify-content:space-between; align-items:flex-start; border-bottom:2px solid #cc33cc; padding-bottom:16px; margin-bottom:20px; }
        .brand { font-size:22px; font-weight:800; color:#512868; }
        .brand span { color:#cc33cc; }
        .meta { text-align:right; font-size:12px; color:#64748b; line-height:1.6; }
        h1 { font-size:16px; margin:0 0 4px; }
        .cards { display:flex; gap:12px; margin-bottom:20px; }
        .card { flex:1; border:1px solid #e2e8f0; border-radius:10px; padding:12px 14px; }
        .card .l { font-size:11px; text-transform:uppercase; letter-spacing:.04em; color:#64748b; font-weight:700; }
        .card .v { font-size:20px; font-weight:800; margin-top:2px; }
        table { width:100%; border-collapse:collapse; font-size:12.5px; }
        th { text-align:left; padding:9px 10px; background:#faf5fb; color:#512868; border-bottom:1px solid #e2e8f0; font-size:11px; text-transform:uppercase; letter-spacing:.03em; }
        td { padding:9px 10px; border-bottom:1px solid #f1f5f9; vertical-align:top; }
        td.r, th.r { text-align:right; font-variant-numeric:tabular-nums; }
        .cause { color:#dc2626; font-size:11px; margin-top:2px; }
        .foot { margin-top:18px; font-size:11px; color:#94a3b8; }
        @media print { body { margin:16mm; } }
      </style></head><body>
      <div class="head">
        <div>
          <div class="brand">Ober<span>track</span></div>
          <div style="font-size:12px;color:#64748b;margin-top:2px;">${isReceipt ? 'Recibo de pago' : 'Comprobante de pagos'}</div>
        </div>
        <div class="meta">
          ${user?.name ? `<div><strong>${escapeHtml(user.name)}</strong></div>` : ''}
          ${user?.email ? `<div>${escapeHtml(user.email)}</div>` : ''}
          <div>Generado: ${new Date().toLocaleDateString('es')}</div>
        </div>
      </div>
      ${
        isReceipt
          ? ''
          : `<div class="cards">
        <div class="card"><div class="l">Total cobrado</div><div class="v">${fmtMoney(summary?.total_paid ?? 0, currency)}</div></div>
        <div class="card"><div class="l">Pendiente</div><div class="v">${fmtMoney(summary?.pending ?? 0, currency)}</div></div>
        <div class="card"><div class="l">Pagos</div><div class="v">${summary?.count ?? 0}</div></div>
      </div>`
      }
      <table>
        <thead><tr><th>Fecha</th><th>Concepto</th><th class="r">Monto</th><th>Estado</th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
      <div class="foot">Obertrack · Documento generado para uso personal del profesional.</div>
      <script>window.onload=function(){window.print()}</script>
      </body></html>`)
    win.document.close()
  }

  // Abre el formulario de soporte con los datos del pago ya cargados.
  const queryPayment = (p: MyPayment) => {
    const subject = `Consulta sobre pago: ${p.description}`
    const message =
      `Hola, tengo una consulta sobre este pago:\n` +
      `• Concepto: ${p.description}\n` +
      `• Monto: ${fmtMoney(p.amount, currency)}\n` +
      `• Fecha: ${fmtDate(p.date)}\n` +
      `• Estado: ${PAYMENT_LABEL[p.status]}` +
      (p.paylist_id ? `\n• Referencia: Lote #${p.paylist_id}` : '')
    const params = new URLSearchParams({ asunto: subject, mensaje: message, modulo: 'Wallet' })
    navigate(`/soporte?${params.toString()}`)
  }

  return (
    <div className={styles.wallet}>
      <header className={styles.header}>
        <div className={styles.heading}>
          <span className={styles.title}>
            <WalletIcon size={26} /> Mi Wallet
          </span>
          <p className={styles.subtitle}>Tus pagos y ganancias. Solo tú ves esta información.</p>
        </div>
        <div className={styles.headerActions}>
          {enabled && summary && payments.length > 0 && (
            <>
              <Button variant="ghost" size="sm" leftIcon={<FileSpreadsheet size={15} />} onClick={exportExcel}>
                Excel
              </Button>
              <Button variant="ghost" size="sm" leftIcon={<FileText size={15} />} onClick={() => exportPdf()}>
                PDF
              </Button>
            </>
          )}
          <Button
            variant="secondary"
            size="sm"
            leftIcon={<RefreshCw size={15} className={walletQ.isFetching ? styles.spin : undefined} />}
            onClick={() => walletQ.refetch()}
            disabled={walletQ.isFetching}
          >
            Actualizar
          </Button>
        </div>
      </header>

      {walletQ.isLoading && (
        <div className={styles.body}>
          <div className={styles.stats}>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} height={108} radius={20} />
            ))}
          </div>
          <Skeleton height={280} radius={20} />
        </div>
      )}

      {!walletQ.isLoading && enabled === false && (
        <div className={styles.body}>
          <div className={styles.notice}>
            <WalletIcon size={18} />
            <span>Tu billetera todavía no está disponible. Vuelve a consultarla más adelante.</span>
          </div>
        </div>
      )}

      {!walletQ.isLoading && walletQ.isError && (
        <div className={styles.body}>
          <div className={styles.notice}>
            <WalletIcon size={18} />
            <span>No pudimos cargar tus pagos en este momento. Intenta de nuevo en unos minutos.</span>
          </div>
        </div>
      )}

      {enabled && summary && (
        <div className={styles.body}>
          {/* Aviso de verificación (KYC) */}
          {rejected.length > 0 && (
            <div className={styles.kyc}>
              <span className={styles.kycIcon}>
                <ShieldAlert size={20} />
              </span>
              <div className={styles.kycText}>
                <strong>Tienes {rejected.length === 1 ? 'un pago en espera' : `${rejected.length} pagos en espera`} por verificación de identidad.</strong>
                <span>Completa tu verificación (KYC) en Ontop para poder recibir estos pagos.</span>
              </div>
              <Button
                variant="primary"
                size="sm"
                onClick={() => window.open('https://app.getontop.com', '_blank', 'noopener')}
              >
                Completar verificación
              </Button>
            </div>
          )}

          {/* Próximo pago estimado */}
          {nextPayment && (
            <div className={styles.nextPay}>
              <span className={styles.nextPayIcon}>
                <CalendarClock size={20} />
              </span>
              <div className={styles.nextPayText}>
                <span className={styles.nextPayLabel}>Próximo pago estimado</span>
                <span className={styles.nextPayDesc}>{nextPayment.description}</span>
              </div>
              <div className={styles.nextPayAmount}>
                <span className={styles.nextPayValue}>{fmtMoney(nextPayment.amount, currency)}</span>
                <span className={styles.nextPayDate}>{fmtDate(nextPayment.date)}</span>
              </div>
            </div>
          )}

          {/* Resumen */}
          <div className={styles.stats}>
            <StatCard
              tone="green"
              icon={<CheckCircle2 size={24} />}
              label="Total cobrado"
              value={fmtMoney(summary.total_paid, currency)}
              sub={
                monthDelta != null ? (
                  <span
                    className={`${styles.trend} ${monthDelta >= 0 ? styles.trendUp : styles.trendDown}`}
                  >
                    {monthDelta >= 0 ? <TrendingUp size={13} /> : <TrendingDown size={13} />}
                    {Math.abs(monthDelta).toFixed(0)}% vs. mes anterior
                  </span>
                ) : (
                  'últimos 12 meses'
                )
              }
            />
            <StatCard
              tone="orange"
              icon={<Clock size={24} />}
              label="Pendiente"
              value={fmtMoney(summary.pending, currency)}
              sub="en proceso"
            />
            <StatCard
              tone="purple"
              icon={<Receipt size={24} />}
              label="Pagos"
              value={String(summary.count)}
              sub="recibidos"
            />
          </div>

          {/* Gráfico mensual + desglose por tipo */}
          <div className={styles.chartsRow}>
            {monthly.length > 0 && (
              <section className={styles.card}>
                <div className={styles.cardHeader}>
                  <h3>Ganancias por mes</h3>
                  <span className={styles.cardSubtitle}>cobrado</span>
                </div>
                <MonthlyChart data={monthly} currency={currency} />
              </section>
            )}
            {breakdown.total > 0 && (
              <section className={styles.card}>
                <div className={styles.cardHeader}>
                  <h3>Desglose por tipo</h3>
                </div>
                <Breakdown acc={breakdown.acc} total={breakdown.total} currency={currency} />
              </section>
            )}
          </div>

          {/* Historial */}
          <section className={styles.card}>
            <div className={styles.cardHeader}>
              <h3>Mis pagos</h3>
              <span className={styles.cardSubtitle}>
                {filtered.length} de {payments.length}
              </span>
            </div>

            {/* Toolbar: filtros + búsqueda */}
            <div className={styles.toolbar}>
              <div className={styles.chips}>
                {FILTERS.map((f) => {
                  const count =
                    f.key === 'all' ? payments.length : payments.filter((p) => f.match(p.status)).length
                  return (
                    <button
                      key={f.key}
                      className={`${styles.chip} ${filter === f.key ? styles.chipActive : ''}`}
                      onClick={() => setFilter(f.key)}
                    >
                      {f.label} <span className={styles.chipCount}>{count}</span>
                    </button>
                  )
                })}
              </div>
              <div className={styles.searchBox}>
                <Search size={15} />
                <input
                  className={styles.searchInput}
                  placeholder="Buscar concepto…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
            </div>

            <div className={styles.list}>
              {filtered.length ? (
                filtered.map((p) => (
                  <PaymentRow key={p.id} payment={p} currency={currency} onClick={() => setDetail(p)} />
                ))
              ) : (
                <div className={styles.empty}>
                  <Receipt size={38} />
                  <p>
                    {payments.length === 0
                      ? 'Todavía no tienes pagos registrados.'
                      : 'Ningún pago coincide con el filtro.'}
                  </p>
                </div>
              )}
            </div>
          </section>
        </div>
      )}

      {/* Detalle */}
      <Modal
        isOpen={!!detail}
        onClose={() => setDetail(null)}
        title="Detalle del pago"
        size="sm"
        footer={
          detail && (
            <div className={styles.detailActions}>
              <Button
                variant="ghost"
                size="sm"
                leftIcon={<FileText size={15} />}
                onClick={() => detail && exportPdf(detail)}
              >
                Recibo PDF
              </Button>
              <Button
                variant="secondary"
                size="sm"
                leftIcon={<HelpCircle size={15} />}
                onClick={() => {
                  const p = detail
                  setDetail(null)
                  if (p) queryPayment(p)
                }}
              >
                ¿Dudas con este pago?
              </Button>
            </div>
          )
        }
      >
        {detail && <PaymentDetail payment={detail} currency={currency} />}
      </Modal>
    </div>
  )
}

function StatCard({
  tone,
  icon,
  label,
  value,
  sub,
}: {
  tone: string
  icon: React.ReactNode
  label: string
  value: string
  sub: React.ReactNode
}) {
  return (
    <div className={styles.stat}>
      <div className={`${styles.statIcon} ${styles[tone]}`}>{icon}</div>
      <div className={styles.statContent}>
        <span className={styles.statLabel}>{label}</span>
        <span className={styles.statValue}>{value}</span>
        <span className={styles.statSub}>{sub}</span>
      </div>
    </div>
  )
}

function MonthlyChart({
  data,
  currency,
}: {
  data: { key: string; label: string; total: number }[]
  currency?: string
}) {
  const max = Math.max(...data.map((d) => d.total), 1)
  return (
    <div className={styles.chart}>
      {data.map((d) => (
        <div key={d.key} className={styles.chartCol}>
          <div className={styles.chartBarWrap}>
            {d.total > 0 && (
              <span className={styles.chartValue}>{fmtMoney(d.total, currency).replace(/\s?US\$/, '')}</span>
            )}
            <div
              className={styles.chartBar}
              style={{ height: `${Math.max((d.total / max) * 100, d.total > 0 ? 6 : 2)}%` }}
              data-empty={d.total === 0 ? 'true' : undefined}
            />
          </div>
          <span className={styles.chartLabel}>{d.label}</span>
        </div>
      ))}
    </div>
  )
}

function Breakdown({
  acc,
  total,
  currency,
}: {
  acc: Record<PayKind, number>
  total: number
  currency?: string
}) {
  const kinds = (Object.keys(acc) as PayKind[]).filter((k) => acc[k] > 0)
  return (
    <div className={styles.breakdown}>
      <div className={styles.breakdownBar}>
        {kinds.map((k) => (
          <div
            key={k}
            className={styles.breakdownSeg}
            style={{ width: `${(acc[k] / total) * 100}%`, background: KIND_COLOR[k] }}
            title={`${k}: ${fmtMoney(acc[k], currency)}`}
          />
        ))}
      </div>
      <div className={styles.breakdownLegend}>
        {kinds.map((k) => (
          <div key={k} className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: KIND_COLOR[k] }} />
            <span className={styles.legendLabel}>{k}</span>
            <span className={styles.legendValue}>{fmtMoney(acc[k], currency)}</span>
            <span className={styles.legendPct}>{Math.round((acc[k] / total) * 100)}%</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function PaymentRow({
  payment,
  currency,
  onClick,
}: {
  payment: MyPayment
  currency?: string
  onClick: () => void
}) {
  const tone = STATUS_TONE[payment.status] ?? 'wait'
  return (
    <button className={styles.row} onClick={onClick}>
      <span className={`${styles.rowIcon} ${styles[`dot-${tone}`]}`}>
        {payment.status === 'SUCCESS' ? (
          <CheckCircle2 size={18} />
        ) : payment.status === 'PENDING' ? (
          <Clock size={18} />
        ) : (
          <XCircle size={18} />
        )}
      </span>
      <div className={styles.rowDetails}>
        <span className={styles.rowTitle}>{payment.description || 'Pago'}</span>
        <span className={styles.rowMeta}>
          {fmtDate(payment.date)}
          {payment.cause && <span className={styles.rowCause}> · {payment.cause}</span>}
        </span>
      </div>
      <span className={styles.rowAmount}>{fmtMoney(payment.amount, currency)}</span>
      <span className={`${styles.pill} ${styles[`pill-${tone}`]}`}>{PAYMENT_LABEL[payment.status]}</span>
    </button>
  )
}

function PaymentDetail({ payment, currency }: { payment: MyPayment; currency?: string }) {
  const tone = STATUS_TONE[payment.status] ?? 'wait'
  return (
    <div className={styles.detail}>
      <div className={styles.detailAmount}>
        <span className={styles.detailValue}>{fmtMoney(payment.amount, currency)}</span>
        <span className={`${styles.pill} ${styles[`pill-${tone}`]}`}>{PAYMENT_LABEL[payment.status]}</span>
      </div>
      <div className={styles.detailRows}>
        <div className={styles.detailRow}>
          <span className={styles.detailLabel}>
            <Receipt size={15} /> Concepto
          </span>
          <span className={styles.detailData}>{payment.description || 'Pago'}</span>
        </div>
        <div className={styles.detailRow}>
          <span className={styles.detailLabel}>
            <Calendar size={15} /> Fecha
          </span>
          <span className={styles.detailData}>{fmtDate(payment.date)}</span>
        </div>
        {payment.paylist_id ? (
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>
              <Hash size={15} /> Referencia
            </span>
            <span className={styles.detailData}>Lote #{payment.paylist_id}</span>
          </div>
        ) : null}
        {payment.cause ? (
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>
              <XCircle size={15} /> Motivo
            </span>
            <span className={`${styles.detailData} ${styles.detailCause}`}>{payment.cause}</span>
          </div>
        ) : null}
      </div>
    </div>
  )
}
