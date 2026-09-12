import { parseDateOnly } from '../utils/date'

// Datos personales que se muestran en la ficha de un profesional y que hasta
// ahora solo vivían en el formulario de edición: existían en la base y no se
// veían en ninguna pantalla. Customer Success los necesita a la vista —la fecha
// de nacimiento para el cumpleaños, los contactos para una emergencia—, y
// Obersuite los manda en cada contratación nueva.

/** Años cumplidos a la fecha dada. null si la fecha no vale o es futura. */
export function ageAt(birthISO: string | null | undefined, now: Date = new Date()): number | null {
  if (!birthISO) return null
  // Fecha de CALENDARIO: se lee sin zona horaria, o al oeste de Greenwich el
  // cumpleaños cae un día antes.
  const b = parseDateOnly(birthISO)
  if (isNaN(b.getTime()) || b > now) return null
  let age = now.getFullYear() - b.getFullYear()
  const beforeBirthday =
    now.getMonth() < b.getMonth() || (now.getMonth() === b.getMonth() && now.getDate() < b.getDate())
  if (beforeBirthday) age -= 1
  return age
}

/**
 * Los contactos de emergencia como lista.
 *
 * Se guardan en un solo texto separado por comas —es lo que escriben los
 * formularios de perfil y lo que junta /hire al recibirlos de Obersuite—.
 * Esta es la ÚNICA regla de lectura: las pantallas que lo partían a mano
 * repetían el mismo split en cuatro sitios.
 */
export function emergencyContacts(raw: string | null | undefined): string[] {
  if (!raw) return []
  return raw.split(',').map(s => s.trim()).filter(Boolean)
}
