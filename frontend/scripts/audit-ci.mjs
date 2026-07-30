#!/usr/bin/env node
/**
 * Candado de dependencias para CI.
 *
 * Hace lo mismo que `npm audit --omit=dev --audit-level=high` (solo lo que se
 * despliega; las herramientas de desarrollo no llegan al usuario), pero permite
 * excepciones declaradas en audit-allowlist.json.
 *
 * Existe porque el caso "alerta alta que no aplica a esta app y sin arreglo
 * hacia delante" deja dos malas salidas: bajar de versión (perdiendo otros
 * arreglos) o apagar la auditoría entera (quedándose ciego ante lo siguiente).
 * Con la lista, el candado sigue cerrado para todo lo demás y cada excepción
 * lleva motivo y fecha de caducidad.
 *
 * Salida: 0 si todo lo alto/crítico está corregido o exceptuado; 1 si aparece
 * algo nuevo, o si una excepción caducó.
 */
import { execSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = join(dirname(fileURLToPath(import.meta.url)), '..')
const BLOCKING = new Set(['high', 'critical'])

function runAudit() {
  try {
    // npm audit sale con código 1 cuando encuentra algo: eso no es un fallo de
    // ejecución, así que se lee stdout igualmente.
    //
    // Comando literal con execSync (y no execFileSync con argumentos): en
    // Windows npm es un .cmd y Node se niega a lanzarlo sin shell. Al ser una
    // cadena fija, sin nada interpolado, no hay riesgo de inyección.
    return execSync('npm audit --omit=dev --json', {
      cwd: frontendDir,
      encoding: 'utf8',
      maxBuffer: 20 * 1024 * 1024,
      stdio: ['ignore', 'pipe', 'pipe'],
    })
  } catch (err) {
    if (err.stdout) return err.stdout
    console.error('No se pudo ejecutar npm audit:', err.message)
    process.exit(1)
  }
}

/** Saca el id GHSA de la URL del aviso ("…/advisories/GHSA-xxxx-yyyy-zzzz"). */
function ghsaFrom(via) {
  const match = /GHSA-[\w-]+/.exec(via?.url ?? '')
  return match ? match[0] : null
}

/** Aplana el informe de npm en una lista de avisos altos/críticos. */
function blockingAdvisories(report) {
  const found = new Map()
  for (const vuln of Object.values(report.vulnerabilities ?? {})) {
    for (const via of vuln.via ?? []) {
      // Las entradas de texto son dependencias intermedias, no avisos propios.
      if (typeof via === 'string') continue
      if (!BLOCKING.has(via.severity)) continue
      const id = ghsaFrom(via)
      if (!id) continue
      found.set(id, {
        id,
        package: via.name ?? vuln.name,
        title: via.title ?? '(sin título)',
        severity: via.severity,
        url: via.url,
      })
    }
  }
  return [...found.values()]
}

const report = JSON.parse(runAudit())
const advisories = blockingAdvisories(report)

const allowlist = JSON.parse(readFileSync(join(frontendDir, 'audit-allowlist.json'), 'utf8'))
const exceptions = new Map((allowlist.excepciones ?? []).map(e => [e.advisory, e]))

const today = new Date().toISOString().slice(0, 10)
const blocked = []
const allowed = []

for (const advisory of advisories) {
  const exception = exceptions.get(advisory.id)
  if (!exception) {
    blocked.push({ advisory, reason: 'sin excepción declarada' })
    continue
  }
  if (exception.revisar_antes && exception.revisar_antes < today) {
    blocked.push({
      advisory,
      reason: `la excepción caducó el ${exception.revisar_antes}: toca revisarla`,
    })
    continue
  }
  allowed.push({ advisory, exception })
}

// Excepciones que ya no hacen falta: no rompen el candado, pero se avisa para
// que la lista no acumule entradas muertas.
const stale = [...exceptions.keys()].filter(id => !advisories.some(a => a.id === id))

if (allowed.length > 0) {
  console.log(`Avisos altos exceptuados (${allowed.length}):`)
  for (const { advisory, exception } of allowed) {
    console.log(`  · ${advisory.id} — ${advisory.package}: ${advisory.title}`)
    console.log(`    motivo: ${exception.motivo}`)
    console.log(`    revisar antes de: ${exception.revisar_antes ?? 'sin fecha'}`)
  }
  console.log('')
}

if (stale.length > 0) {
  console.log(`Excepciones que ya se pueden borrar de audit-allowlist.json: ${stale.join(', ')}\n`)
}

if (blocked.length > 0) {
  console.error(`Vulnerabilidades que bloquean el despliegue (${blocked.length}):`)
  for (const { advisory, reason } of blocked) {
    console.error(`  · ${advisory.id} [${advisory.severity}] — ${advisory.package}: ${advisory.title}`)
    console.error(`    ${advisory.url}`)
    console.error(`    ${reason}`)
  }
  console.error('\nArréglalas, o si no aplican a esta app, decláralas en frontend/audit-allowlist.json con motivo y fecha.')
  process.exit(1)
}

console.log('Sin vulnerabilidades altas o críticas pendientes en lo que se despliega.')
