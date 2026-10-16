import { safeReturnTo } from './returnTo'

describe('safeReturnTo', () => {
  it('acepta la autorización del MCP', () => {
    const v = '/api/mcp/oauth/authorize?client_id=obc_x&state=abc'
    expect(safeReturnTo(v)).toBe(v)
  })

  it('rechaza cualquier otro destino', () => {
    for (const v of [
      null,
      '',
      '/dashboard',
      'https://evil.com/api/mcp/oauth/authorize?x=1',
      '//evil.com/api/mcp/oauth/authorize?x=1',
      '/api/mcp/oauth/authorizeX?x=1',
      '/api/mcp/oauth/token?x=1',
      '/api/mcp/oauth/authorize?x=1\\evil',
    ]) {
      expect(safeReturnTo(v)).toBeNull()
    }
  })
})
