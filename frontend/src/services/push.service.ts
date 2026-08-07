import api from './client'

/** Cliente de Web Push: clave VAPID y alta/baja de la suscripción del navegador. */
export const pushService = {
  getPublicKey: async (): Promise<string> => {
    const { data } = await api.get<{ public_key: string }>('/notifications/push/key')
    return data.public_key || ''
  },
  subscribe: async (subscription: PushSubscriptionJSON): Promise<void> => {
    await api.post('/notifications/push/subscriptions', subscription)
  },
  unsubscribe: async (endpoint: string): Promise<void> => {
    await api.delete('/notifications/push/subscriptions', { data: { endpoint } })
  },
}
