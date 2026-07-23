import api from './client'

// Vista PERSONAL de la billetera: cada profesional consulta solo sus propios
// pagos (solo lectura). El backend filtra por el email del usuario autenticado.

export type PaymentStatus = 'PENDING' | 'SUCCESS' | 'FAILURE' | 'REJECTED' | 'ERROR'

export interface MyPayment {
  id: number
  description: string
  amount: number
  status: PaymentStatus
  cause?: string
  paylist_id?: number
  /** Fecha del pago en ISO 8601 (puede faltar si Ontop no la expone). */
  date?: string
}

export interface EarningsSummary {
  total_paid: number
  pending: number
  count: number
  currency: string
  payments: MyPayment[]
}

export interface MyWalletResponse {
  enabled: boolean
  summary?: EarningsSummary
}

// ─────────────────────────────────────────────────────────────────────────
// TEMP · MODO MOCK
// Datos de ejemplo para revisar la interfaz mientras no hay credenciales de
// Ontop. Poné USE_MOCK en false (o borrá este bloque) cuando el endpoint real
// /me/wallet esté conectado.
// ─────────────────────────────────────────────────────────────────────────
const USE_MOCK = true

const MOCK_WALLET: MyWalletResponse = {
  enabled: true,
  summary: {
    total_paid: 2220,
    pending: 320,
    count: 8,
    currency: 'USD',
    payments: [
      { id: 8, description: 'Pago quincena 1 de julio', amount: 420, status: 'SUCCESS', date: '2026-07-15', paylist_id: 48 },
      { id: 7, description: 'Pago quincena 2 de junio', amount: 420, status: 'SUCCESS', date: '2026-06-30', paylist_id: 45 },
      { id: 6, description: 'Bono de desempeño', amount: 200, status: 'SUCCESS', date: '2026-06-20', paylist_id: 44 },
      { id: 5, description: 'Pago quincena 1 de junio', amount: 420, status: 'SUCCESS', date: '2026-06-15', paylist_id: 42 },
      { id: 4, description: 'Pago quincena 2 de mayo', amount: 380, status: 'SUCCESS', date: '2026-05-31', paylist_id: 39 },
      { id: 3, description: 'Pago quincena 1 de mayo', amount: 380, status: 'SUCCESS', date: '2026-05-15', paylist_id: 37 },
      { id: 2, description: 'Pago quincena 1 de agosto', amount: 320, status: 'PENDING', date: '2026-07-31', paylist_id: 50 },
      {
        id: 1,
        description: 'Reembolso de equipo',
        amount: 120,
        status: 'REJECTED',
        cause: 'Verificación de identidad pendiente',
        date: '2026-07-10',
        paylist_id: 47,
      },
    ],
  },
}

export const walletService = {
  async myWallet(): Promise<MyWalletResponse> {
    if (USE_MOCK) {
      // Pequeño delay para simular la carga real.
      await new Promise((r) => setTimeout(r, 400))
      return MOCK_WALLET
    }
    const { data } = await api.get<MyWalletResponse>('/me/wallet')
    return data
  },
}
