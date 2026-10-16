import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { NotificationProvider } from '../../context/NotificationContext'
import ConnectionGuide, { type ConnectionGuideProps } from './ConnectionGuide'
import type { McpConnection } from '../../services/mcp.service'

const URL = 'https://obertrack.com/api/mcp'

function renderGuide(over: Partial<ConnectionGuideProps> = {}) {
  const props: ConnectionGuideProps = {
    client: 'claude',
    url: URL,
    connections: [],
    personalTokens: [],
    newToken: null,
    creatingToken: false,
    onCreateToken: vi.fn(),
    ...over,
  }
  render(<NotificationProvider><ConnectionGuide {...props} /></NotificationProvider>)
  return props
}

const conn = (client_name: string): McpConnection => ({
  id: 1, client_name, personal: false, created_at: '', last_used_at: null, expires_at: '',
})

describe('ConnectionGuide', () => {
  it('Claude: pasos con el nombre y la URL para copiar, y la pregunta de prueba', () => {
    renderGuide()
    expect(screen.getByRole('button', { name: 'Copiar el nombre' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copiar la URL' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Personalizar → Conectores/ })).toHaveAttribute('href', 'https://claude.ai/customize/connectors')
    expect(screen.getByText('¿Quién soy en Obertrack y qué puedo consultar?')).toBeInTheDocument()
    expect(screen.queryByText(/URL local de desarrollo/)).not.toBeInTheDocument()
  })

  it('marca el asistente como conectado si ya aparece en Mis conexiones', () => {
    renderGuide({ client: 'claude', connections: [conn('Claude')] })
    expect(screen.getByText(/Ya tienes conectado «Claude»/)).toBeInTheDocument()
  })

  it('no confunde una conexión de otro asistente', () => {
    renderGuide({ client: 'chatgpt', connections: [conn('Claude')] })
    expect(screen.queryByText(/Ya tienes conectado/)).not.toBeInTheDocument()
  })

  it('avisa si la URL es local: los asistentes en la nube no llegan a ella', () => {
    renderGuide({ url: 'http://localhost:8081/api/mcp' })
    expect(screen.getByText(/URL local de desarrollo/)).toBeInTheDocument()
  })

  it('Gemini CLI: genera el token desde la guía', async () => {
    const props = renderGuide({ client: 'gemini' })
    await userEvent.click(screen.getByRole('button', { name: /Generar token para Gemini CLI/ }))
    expect(props.onCreateToken).toHaveBeenCalledTimes(1)
  })

  it('Gemini CLI: con el token creado, el comando sale completo', () => {
    renderGuide({ client: 'gemini', newToken: 'obs_pt_abc123' })
    expect(
      screen.getByText(`gemini mcp add --scope user --transport http --header "Authorization: Bearer obs_pt_abc123" obertrack ${URL}`),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Generar token/ })).not.toBeInTheDocument()
  })
})
