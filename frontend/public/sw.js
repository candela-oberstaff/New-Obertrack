/*
 * Service worker de Obertrack: recibe Web Push con la pestaña cerrada y abre
 * (o enfoca) la app al hacer clic en la notificación. El payload lo cifra el
 * backend (webpush_service.go) con el formato {title, body, url}.
 */

self.addEventListener('push', (event) => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = { body: event.data ? event.data.text() : '' }
  }
  const title = data.title || 'Obertrack'
  event.waitUntil(
    self.registration.showNotification(title, {
      body: data.body || '',
      icon: '/logos/Isotipo_Color.png',
      badge: '/logos/Isotipo_Color.png',
      data: { url: data.url || '/' },
    })
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data && event.notification.data.url) || '/'
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((list) => {
      for (const client of list) {
        if ('focus' in client) {
          if ('navigate' in client) client.navigate(url)
          return client.focus()
        }
      }
      return clients.openWindow(url)
    })
  )
})
