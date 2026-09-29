import { useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { postToCrm } from './crmBridge'
import { EmbedMessage } from './EmbedMessage'

// /embed/error?code=<código>: adonde redirige el canje cuando rechaza el token
// (contrato CRM, §4 y §7). Avisa al CRM con el código y lo muestra.
export default function EmbedError() {
  const [params] = useSearchParams()
  const code = params.get('code') ?? ''

  useEffect(() => {
    postToCrm({ type: 'obertrack:error', code })
  }, [code])

  return <EmbedMessage code={code} />
}
