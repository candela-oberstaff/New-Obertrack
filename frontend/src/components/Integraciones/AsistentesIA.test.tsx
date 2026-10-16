import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { NotificationProvider } from '../../context/NotificationContext'
import { ConfirmProvider } from '../ui/ConfirmProvider'
import AsistentesIA from './AsistentesIA'

vi.mock('../../services/mcp.service', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../services/mcp.service')>()
  return {
    ...actual,
    mcpService: {
      ...actual.mcpService,
      getStatus: vi.fn().mockResolvedValue({
        enabled: true, paused: false, server_url: 'https://obertrack.com/api/mcp', role: 'superadmin',
        role_label: 'SuperAdmin', scope: 'Todo', quota: { daily_calls: 1500, max_rows: 50 }, used_today: 0,
      }),
      listConnections: vi.fn().mockResolvedValue([]),
      listTokens: vi.fn().mockResolvedValue([]),
    },
  }
})

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <NotificationProvider>
        <ConfirmProvider>
          <AsistentesIA />
        </ConfirmProvider>
      </NotificationProvider>
    </QueryClientProvider>,
  )
}

describe('AsistentesIA: guía en acordeón', () => {
  it('arranca cerrada y se abre, cambia y se cierra al tocar las tarjetas', async () => {
    renderPage()
    const claude = await screen.findByRole('button', { name: /Claude/ })
    const grok = screen.getByRole('button', { name: /Grok/ })

    expect(claude).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Cómo conectar Claude')).not.toBeInTheDocument()

    await userEvent.click(claude)
    expect(claude).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Cómo conectar Claude')).toBeInTheDocument()

    await userEvent.click(grok)
    expect(claude).toHaveAttribute('aria-expanded', 'false')
    expect(grok).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Cómo conectar Grok')).toBeInTheDocument()

    await userEvent.click(grok)
    expect(grok).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('region', { hidden: true })).toHaveAttribute('inert')
  })

  it('el botón Ocultar cierra la guía', async () => {
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: /ChatGPT/ }))
    await userEvent.click(screen.getByRole('button', { name: /Ocultar/ }))
    expect(screen.getByRole('button', { name: /ChatGPT/ })).toHaveAttribute('aria-expanded', 'false')
  })
})
