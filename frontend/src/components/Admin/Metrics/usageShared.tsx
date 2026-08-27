import React from 'react';
import { Activity, Minus, TrendingDown, TrendingUp } from 'lucide-react';
import type { ModuleUsage } from '../../../services/usage.service';
import styles from './UsageTab.module.css';

/**
 * Piezas comunes a las dos pantallas que enseñan uso: el panel global de
 * Métricas y la pestaña Uso de la ficha de una empresa.
 *
 * Viven aquí y no duplicadas en cada una porque son las que fijan el
 * VOCABULARIO: qué se llama "Tareas", a partir de cuántos puntos una variación
 * deja de ser ruido, cuándo una empresa está "en riesgo". Dos copias de eso se
 * separan a la primera corrección y acabas con dos pantallas que discrepan
 * sobre los mismos datos —que es justo lo que destruye la confianza en un
 * panel de métricas—.
 */

/**
 * Nombres de los módulos tal y como los llama la gente que usa la app. El
 * backend ya devuelve la clave en castellano; aquí solo se le pone la mayúscula
 * y se corrigen los casos donde la clave de ruta y el nombre del menú difieren.
 */
export const MODULE_LABELS: Record<string, string> = {
  app: 'Aplicación (global)',
  chat: 'Chat',
  tareas: 'Tareas',
  horas: 'Horas',
  sesiones: 'Sesiones',
  soporte: 'Soporte y tickets',
  encuestas: 'Encuestas',
  novedades: 'Novedades',
  admin: 'Administración',
  empresa: 'Panel de empresa',
  usuarios: 'Fichas de personas',
  correos: 'Correos',
  testimonios: 'Testimonios',
  automatizaciones: 'Automatizaciones',
  perfil: 'Perfil',
  induccion: 'Inducción',
  wallet: 'Wallet',
  reportes: 'Reportes',
};

export const moduleLabel = (key: string) => MODULE_LABELS[key] ?? key;

/**
 * Semáforo por porcentaje de uso. Los cortes son los que usa el equipo al
 * hablar: o la usa la mayoría, o la usa una parte, o no la usa nadie —y ese
 * último caso es el que hay que ver primero—.
 */
export function healthOf(rate: number, activeUsers: number) {
  if (activeUsers === 0) return { label: 'Sin uso', cls: styles.badgeDead };
  if (rate >= 60) return { label: 'Activa', cls: styles.badgeGood };
  if (rate >= 25) return { label: 'Parcial', cls: styles.badgeWarn };
  return { label: 'En riesgo', cls: styles.badgeRisk };
}

/**
 * Delta en puntos porcentuales. Solo se pinta cuando el contador cubre entero
 * el período anterior: con media ventana medida, la "caída" sería el hueco de
 * datos, y una flecha roja falsa quema la confianza en todo el panel.
 */
export const Delta: React.FC<{ value: number; comparable: boolean }> = ({ value, comparable }) => {
  if (!comparable) return null;
  // Menos de un punto es ruido de redondeo, no un movimiento.
  if (Math.abs(value) < 1) {
    return (
      <span className={`${styles.delta} ${styles.deltaFlat}`} title="Sin cambio respecto al período anterior">
        <Minus size={12} /> igual
      </span>
    );
  }
  const up = value > 0;
  return (
    <span
      className={`${styles.delta} ${up ? styles.deltaUp : styles.deltaDown}`}
      title={`${up ? 'Subió' : 'Bajó'} ${Math.abs(value).toFixed(1)} puntos frente al período anterior`}
    >
      {up ? <TrendingUp size={12} /> : <TrendingDown size={12} />}
      {up ? '+' : '−'}{Math.abs(value).toFixed(1)} pts
    </span>
  );
};

/** Lista de barras de "% de uso por módulo", con la muesca del período previo. */
export const ModuleBars: React.FC<{ modules: ModuleUsage[]; comparable: boolean }> = ({
  modules,
  comparable,
}) => {
  // "app" es el total y ya sale en las tarjetas de arriba; repetirlo como la
  // barra más larga de la lista solo aplastaría a las demás.
  const list = modules.filter((m) => m.module !== 'app');
  const maxRate = Math.max(1, ...list.map((m) => Math.max(m.rate, m.prev_rate)));

  if (list.length === 0) {
    return <p className={styles.empty}>Sin actividad registrada en el período.</p>;
  }

  return (
    <div className={styles.moduleList}>
      {list.map((m) => (
        <div key={m.module} className={styles.moduleRow}>
          <div className={styles.moduleTop}>
            <span className={styles.moduleName}>{moduleLabel(m.module)}</span>
            <span className={styles.moduleValue}>
              <Delta value={m.delta} comparable={comparable} />
              {m.rate.toFixed(1)}% <small>{m.users} pers.</small>
            </span>
          </div>
          <div className={styles.bar}>
            {/* La marca del período anterior queda como muesca: sin una
                referencia visible, una barra corta no dice si viene subiendo
                o cayendo. */}
            {comparable && m.prev_rate > 0 && (
              <span className={styles.barMark} style={{ left: `${(m.prev_rate / maxRate) * 100}%` }} />
            )}
            <div className={styles.barFill} style={{ width: `${(m.rate / maxRate) * 100}%` }} />
          </div>
        </div>
      ))}
    </div>
  );
};

/**
 * Los dos avisos que impiden leer mal un número recién estrenado: cuánto lleva
 * midiendo el contador, y por qué todavía no hay flechas de variación.
 */
export const UsageNotice: React.FC<{
  trackingSince: Date | null;
  trackedDays: number;
  days: number;
  comparable: boolean;
}> = ({ trackingSince, trackedDays, days, comparable }) => {
  if (trackedDays < days) {
    return (
      <div className={styles.notice}>
        <Activity size={16} />
        <span>
          {trackingSince
            ? `El registro de uso lleva ${trackedDays} ${trackedDays === 1 ? 'día' : 'días'} activo. Los porcentajes solo cubren ese tramo, no los ${days} días del período.`
            : 'Todavía no hay actividad registrada. Los porcentajes se llenan a medida que la gente use la app.'}
        </span>
      </div>
    );
  }
  if (!comparable) {
    return (
      <div className={styles.notice}>
        <Activity size={16} />
        <span>
          Todavía no se puede comparar con el período anterior: harían falta {days * 2} días medidos
          y llevamos {trackedDays}. Las variaciones aparecen cuando el contador cubra las dos
          ventanas enteras.
        </span>
      </div>
    );
  }
  return null;
};

/** Cuántos días lleva midiendo el contador, para los avisos de arriba. */
export function trackedDaysFrom(trackingSince: Date | null): number {
  if (!trackingSince) return 0;
  return Math.max(1, Math.round((Date.now() - trackingSince.getTime()) / 86400000) + 1);
}

export const Pager: React.FC<{
  total: number;
  page: number;
  shown: number;
  noun: string;
  pageSize?: number;
  onPage: (fn: (p: number) => number) => void;
}> = ({ total, page, shown, noun, pageSize = 25, onPage }) => (
  <div className={styles.pager}>
    <span>
      {total} {noun}{total === 1 ? '' : 's'}
    </span>
    <div>
      <button disabled={page <= 1} onClick={() => onPage((p) => p - 1)}>Anterior</button>
      <button disabled={shown < pageSize || page * pageSize >= total} onClick={() => onPage((p) => p + 1)}>
        Siguiente
      </button>
    </div>
  </div>
);
