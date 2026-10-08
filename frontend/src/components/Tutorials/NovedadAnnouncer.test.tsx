import { render, screen, act, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { NovedadAnnouncer } from './NovedadAnnouncer'
import { clearAnnouncerSession } from './announcerSession'
import { tutorialService } from '../../services/api'
import type { Tutorial } from '../../types'

vi.mock('../../services/api', () => ({
  tutorialService: {
    getPending: vi.fn(),
    recordShow: vi.fn(),
    recordView: vi.fn(),
    recordClick: vi.fn(),
  },
}))

const { FAKE_USER } = vi.hoisted(() => ({ FAKE_USER: { id: 7, name: 'Ana' } }))

vi.mock('../../context/AuthContext', () => ({
  useAuth: () => ({ user: FAKE_USER }),
}))

const NOVEDAD = {
  id: 42,
  title: 'Cambio en el registro de horas',
  description: 'Resumen',
  content_type: 'texto',
  google_drive_url: '',
  image_url: '',
  body: '<p>Contenido</p>',
  icon_name: 'PlayCircle',
  category: 'General',
  audience: 'all',
  target: { company_ids: [], countries: [], group_ids: [], managers_only: false },
  duration_min: 0,
  order_index: 0,
  is_active: true,
  announce_days: 2,
  announce_max_shows: 3,
  cta_label: '',
  cta_url: '',
  require_ack: false,
  created_by: 1,
  created_at: '2026-08-20T09:00:00Z',
  updated_at: '2026-08-20T09:00:00Z',
} as unknown as Tutorial

function renderAnnouncer() {
  // Cada montaje usa su propio cliente: es lo que simula una recarga de la
  // página, que es justo el caso que hay que cubrir (la persona no cierra
  // sesión, solo vuelve a entrar).
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <NovedadAnnouncer />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  // Cada prueba empieza en un inicio de sesión nuevo.
  clearAnnouncerSession()
  vi.mocked(tutorialService.getPending).mockResolvedValue([NOVEDAD])
  vi.mocked(tutorialService.recordShow).mockResolvedValue(undefined)
  vi.mocked(tutorialService.recordView).mockResolvedValue(undefined)
})

afterEach(cleanup)

describe('NovedadAnnouncer · conteo de apariciones', () => {
  it('registra la aparición una sola vez al mostrar el aviso', async () => {
    renderAnnouncer()
    expect(await screen.findByText('Cambio en el registro de horas')).toBeInTheDocument()

    expect(tutorialService.recordShow).toHaveBeenCalledTimes(1)
    expect(tutorialService.recordShow).toHaveBeenCalledWith(42)
  })

  it('no vuelve a contar mientras el aviso sigue en pantalla', async () => {
    renderAnnouncer()
    await screen.findByText('Cambio en el registro de horas')

    // El aviso se repregunta solo cada minuto: que la lista vuelva a llegar no
    // es una aparición nueva, es la misma que sigue delante.
    await act(async () => {
      window.dispatchEvent(new CustomEvent('novedad-published'))
      await Promise.resolve()
    })

    expect(tutorialService.recordShow).toHaveBeenCalledTimes(1)
  })

  it('recargar la página no gasta otra aparición', async () => {
    renderAnnouncer()
    await screen.findByText('Cambio en el registro de horas')
    expect(tutorialService.recordShow).toHaveBeenCalledTimes(1)

    // Un F5 monta el componente de nuevo, pero sigue siendo el mismo inicio
    // de sesión: no es una segunda aparición.
    cleanup()
    renderAnnouncer()
    await screen.findByText('Cambio en el registro de horas')

    expect(tutorialService.recordShow).toHaveBeenCalledTimes(1)
  })

  it('cuenta otra vez en un inicio de sesión nuevo', async () => {
    renderAnnouncer()
    await screen.findByText('Cambio en el registro de horas')

    // Salir y volver a entrar limpia la sesión del aviso.
    cleanup()
    clearAnnouncerSession()
    renderAnnouncer()
    await screen.findByText('Cambio en el registro de horas')

    expect(tutorialService.recordShow).toHaveBeenCalledTimes(2)
  })

  it('cerrada, no vuelve en la misma sesión aunque el servidor la siga devolviendo', async () => {
    renderAnnouncer()
    await screen.findByText('Cambio en el registro de horas')
    await act(async () => { screen.getAllByRole('button').find(b => /cerrar|entendido/i.test(b.textContent || b.getAttribute('aria-label') || ''))?.click() })

    cleanup()
    renderAnnouncer()
    await act(async () => { await Promise.resolve() })
    expect(screen.queryByText('Cambio en el registro de horas')).not.toBeInTheDocument()
  })

  it('deja de aparecer cuando el servidor ya no la devuelve', async () => {
    // Alcanzado el tope, el servidor la excluye de los pendientes: el aviso no
    // se pinta y no se cuenta ninguna aparición más.
    vi.mocked(tutorialService.getPending).mockResolvedValue([])
    renderAnnouncer()

    await act(async () => { await Promise.resolve() })

    expect(screen.queryByText('Cambio en el registro de horas')).not.toBeInTheDocument()
    expect(tutorialService.recordShow).not.toHaveBeenCalled()
  })
})
