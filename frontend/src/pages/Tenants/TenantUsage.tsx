import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Activity, Download, Radio, UserPlus, Users } from 'lucide-react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { usageService, type NeverActiveUser, type PersonUsage } from '../../services/usage.service';
import { adminService } from '../../services/admin.service';
import {
  Delta,
  healthOf,
  ModuleBars,
  moduleLabel,
  Pager,
  trackedDaysFrom,
  UsageNotice,
} from '../../components/Admin/Metrics/usageShared';
import { healthSignal, HEALTH_COLOR } from './accountHealth';
import { Select } from '../../components/ui/Select';
import { Button, Modal } from '../../components/ui';
import styles from '../../components/Admin/Metrics/UsageTab.module.css';

interface TenantUsageProps {
  companyId: number;
  companyName: string;
}

/**
 * Uso de la app de UNA empresa, dentro de su ficha.
 *
 * Es la misma pregunta del panel global de Métricas —¿la usan?— hecha desde el
 * sitio donde Customer Success prepara una llamada. Comparte hoja de estilos y
 * piezas con aquel (usageShared) a propósito: dos pantallas que enseñan el
 * mismo dato con distinta pinta se leen como dos datos distintos.
 *
 * Aquí NO se ofrece el interruptor de "solo clientes": dentro de una empresa
 * todos sus miembros son clientes, y el conmutador solo sembraría la duda de si
 * el número que se está mirando incluye a alguien de casa.
 */
export function TenantUsage({ companyId, companyName }: TenantUsageProps) {
  const navigate = useNavigate();
  const [days, setDays] = useState(30);
  const [board, setBoard] = useState<'people' | 'activation'>('people');
  const [page, setPage] = useState(1);
  const [exporting, setExporting] = useState(false);
  const [followUpFor, setFollowUpFor] = useState<{ id: number; name: string } | null>(null);
  const [followUpStatus, setFollowUpStatus] = useState('contacted');
  const [followUpNote, setFollowUpNote] = useState('');

  const { data: summary, isLoading } = useQuery({
    queryKey: ['tenant-usage', companyId, days],
    queryFn: () => usageService.getSummary(days, 'clients', companyId),
    enabled: companyId > 0,
  });

  const { data: people } = useQuery({
    queryKey: ['tenant-usage-people', companyId, days, page],
    queryFn: () => usageService.getPeople({ days, companyId, page, limit: 25 }),
    enabled: companyId > 0 && board === 'people',
    placeholderData: (prev) => prev,
  });

  const { data: activation } = useQuery({
    queryKey: ['tenant-usage-activation', companyId, page],
    queryFn: () => usageService.getActivation('clients', page, 25, companyId),
    enabled: companyId > 0,
    placeholderData: (prev) => prev,
  });

  const { data: onlineIds = [] } = useQuery({
    queryKey: ['usage-online'],
    queryFn: () => usageService.getOnline(),
    refetchInterval: 20_000,
  });
  const onlineSet = useMemo(() => new Set(onlineIds), [onlineIds]);

  const followUp = useMutation({
    mutationFn: (payload: { user_id: number; status: string; note: string }) =>
      adminService.createFollowUp({
        user_id: payload.user_id,
        kind: 'inactivity',
        status: payload.status,
        note: payload.note,
      }),
    onSuccess: () => {
      setFollowUpFor(null);
      setFollowUpNote('');
      setFollowUpStatus('contacted');
    },
  });

  const overview = summary?.overview;
  const comparable = !!overview?.comparable;
  const trackingSince = overview?.tracking_since ? new Date(overview.tracking_since) : null;
  const trackedDays = trackedDaysFrom(trackingSince);

  const trend = (summary?.trend ?? []).map((d) => ({ name: d.day.slice(5), users: d.users }));

  // Conectados de ESTA empresa, no de la plataforma: el contador global en una
  // ficha se leería como si toda esa gente fuera del cliente que se mira.
  const onlineHere = (people?.data ?? []).filter((p) => onlineSet.has(p.user_id)).length;

  const health = healthOf(overview?.adoption_rate ?? 0, overview?.active_users ?? 0);
  const activationTotal = activation?.total ?? 0;

  const handleExport = async () => {
    setExporting(true);
    try {
      await usageService.exportBoard(board, { days, companyId });
    } finally {
      setExporting(false);
    }
  };

  const switchBoard = (next: 'people' | 'activation') => {
    setBoard(next);
    setPage(1);
  };

  if (isLoading && !summary) {
    return (
      <div className={styles.loading}>
        <Activity size={40} className={styles.spin} />
        <p>Calculando el uso de {companyName}...</p>
      </div>
    );
  }

  return (
    <div className={styles.wrap}>
      <UsageNotice
        trackingSince={trackingSince}
        trackedDays={trackedDays}
        days={days}
        comparable={comparable}
      />

      <div className={styles.scopeRow}>
        <Select
          value={days}
          onChange={(v) => setDays(Number(v))}
          options={[
            { value: 7, label: 'Últimos 7 días' },
            { value: 30, label: 'Últimos 30 días' },
            { value: 90, label: 'Últimos 90 días' },
          ]}
        />
        <span className={styles.scopeHint}>
          Cuenta a las {overview?.eligible_users ?? 0} personas activas de {companyName}.
        </span>
      </div>

      <div className={styles.statsRow}>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#ede9fe', color: '#6d28d9' }}>
            <Users size={24} />
          </div>
          <div className={styles.statInfo}>
            <label>Uso de la app</label>
            <h3>
              {(overview?.adoption_rate ?? 0).toFixed(0)}%
              <Delta value={overview?.adoption_delta ?? 0} comparable={comparable} />
            </h3>
            <small>
              {overview?.active_users ?? 0} de {overview?.eligible_users ?? 0} personas la abrieron
            </small>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#dcfce7', color: '#15803d' }}>
            <Radio size={24} />
          </div>
          <div className={styles.statInfo}>
            <label>Conectados ahora</label>
            <h3>
              <span className={styles.livePulse} />
              {onlineHere}
            </h3>
            <small>De esta empresa, con la app abierta</small>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#fee2e2', color: '#b91c1c' }}>
            <UserPlus size={24} />
          </div>
          <div className={styles.statInfo}>
            <label>Sin estrenar</label>
            <h3>{activationTotal}</h3>
            <small>Cuentas que nunca han entrado</small>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#dbeafe', color: '#1d4ed8' }}>
            <Activity size={24} />
          </div>
          <div className={styles.statInfo}>
            <label>Estado</label>
            <h3>
              <span className={`${styles.badge} ${health.cls}`} style={{ fontSize: 14 }}>{health.label}</span>
            </h3>
            <small>
              {(overview?.avg_active_days ?? 0).toFixed(1)} días de uso por persona activa
            </small>
          </div>
        </div>
      </div>

      <div className={styles.chartsGrid}>
        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <h4>Personas activas por día</h4>
            <p>Cuánta gente de {companyName} abrió la app cada día</p>
          </div>
          <ResponsiveContainer width="100%" height={240}>
            <AreaChart data={trend}>
              <defs>
                <linearGradient id="tenantUsageFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <YAxis allowDecimals={false} tick={{ fontSize: 11, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <Tooltip formatter={(v: any) => [`${v} personas`, 'Activas']} />
              <Area type="monotone" dataKey="users" stroke="#8b5cf6" strokeWidth={2} fill="url(#tenantUsageFill)" />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <h4>Qué usan</h4>
            <p>% de su gente que tocó cada módulo</p>
          </div>
          <ModuleBars modules={summary?.modules ?? []} comparable={comparable} />
        </div>
      </div>

      <div className={styles.card}>
        <div className={styles.boardHeader}>
          <div className={styles.scopeToggle}>
            <button className={board === 'people' ? styles.active : ''} onClick={() => switchBoard('people')}>
              <Users size={15} /> Su gente
            </button>
            <button className={board === 'activation' ? styles.active : ''} onClick={() => switchBoard('activation')}>
              <UserPlus size={15} /> Sin estrenar
              {activationTotal > 0 && <span className={styles.tabCount}>{activationTotal}</span>}
            </button>
          </div>
          <Button variant="secondary" size="sm" leftIcon={<Download size={15} />} loading={exporting} onClick={handleExport}>
            Excel
          </Button>
        </div>

        {board === 'people' ? (
          <>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Persona</th>
                    <th>Estado</th>
                    <th>Días con uso</th>
                    <th>Última señal</th>
                    <th>Módulos que usa</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {(people?.data ?? []).length === 0 && (
                    <tr>
                      <td colSpan={6} className={styles.empty}>Esta empresa no tiene personas activas.</td>
                    </tr>
                  )}
                  {(people?.data ?? []).map((p: PersonUsage) => {
                    const online = onlineSet.has(p.user_id);
                    const signal = healthSignal(p.last_active);
                    return (
                      <tr
                        key={p.user_id}
                        className={styles.clickable}
                        onClick={() => navigate(`/admin/tenants/${companyId}/employees/${p.user_id}`)}
                        title="Abrir la ficha de la persona"
                      >
                        <td>
                          <div className={styles.person}>
                            <span className={styles.strong}>{p.name}</span>
                            <small>{p.email}</small>
                          </div>
                        </td>
                        <td>
                          <span className={`${styles.presence} ${online ? styles.on : styles.off}`}>
                            <span className={styles.dot} />
                            {online ? 'En línea' : 'Desconectado'}
                          </span>
                        </td>
                        <td>
                          {p.active_days} <small className={styles.muted}>/ {days}</small>
                        </td>
                        <td style={{ color: HEALTH_COLOR[signal.level], fontWeight: 600 }}>{signal.label}</td>
                        <td>
                          <div className={styles.chips}>
                            {p.modules
                              ? p.modules.split(',').map((m) => (
                                  <span key={m} className={styles.chip}>{moduleLabel(m)}</span>
                                ))
                              : <span className={styles.muted}>—</span>}
                          </div>
                        </td>
                        <td>
                          <button
                            className={styles.rowAction}
                            onClick={(e) => { e.stopPropagation(); setFollowUpFor({ id: p.user_id, name: p.name }); }}
                          >
                            Anotar gestión
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            <Pager total={people?.total ?? 0} page={page} shown={people?.data.length ?? 0} onPage={setPage} noun="persona" />
          </>
        ) : (
          <>
            <p className={styles.boardIntro}>
              Cuentas de {companyName} que <strong>nunca</strong> han aparecido en el contador de uso:
              se les dio acceso y no lo estrenaron. Las marcadas como <em>confirmado</em> se dieron de
              alta después de que empezáramos a medir, así que son un hecho.
            </p>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Persona</th>
                    <th>Alta</th>
                    <th>Días sin estrenar</th>
                    <th>Certeza</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {(activation?.data ?? []).length === 0 && (
                    <tr>
                      <td colSpan={5} className={styles.empty}>
                        Todo el mundo de esta empresa ha entrado alguna vez. Buena señal.
                      </td>
                    </tr>
                  )}
                  {(activation?.data ?? []).map((u: NeverActiveUser) => (
                    <tr
                      key={u.user_id}
                      className={styles.clickable}
                      onClick={() => navigate(`/admin/tenants/${companyId}/employees/${u.user_id}`)}
                      title="Abrir la ficha de la persona"
                    >
                      <td>
                        <div className={styles.person}>
                          <span className={styles.strong}>{u.name}</span>
                          <small>{u.email}</small>
                        </div>
                      </td>
                      <td className={styles.muted}>{new Date(u.created_at).toLocaleDateString()}</td>
                      <td className={styles.strong}>{u.days_since}</td>
                      <td>
                        <span className={`${styles.badge} ${u.certain ? styles.badgeDead : styles.badgeWarn}`}>
                          {u.certain ? 'Confirmado' : 'Sin datos previos'}
                        </span>
                      </td>
                      <td>
                        <button
                          className={styles.rowAction}
                          onClick={(e) => { e.stopPropagation(); setFollowUpFor({ id: u.user_id, name: u.name }); }}
                        >
                          Anotar gestión
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Pager total={activationTotal} page={page} shown={activation?.data.length ?? 0} onPage={setPage} noun="cuenta" />
          </>
        )}
      </div>

      <Modal
        isOpen={!!followUpFor}
        onClose={() => setFollowUpFor(null)}
        title={`Anotar gestión · ${followUpFor?.name ?? ''}`}
        size="sm"
        isDirty={followUpNote.trim().length > 0}
        footer={
          <>
            <Button variant="ghost" onClick={() => setFollowUpFor(null)}>Cancelar</Button>
            <Button
              loading={followUp.isPending}
              onClick={() =>
                followUpFor &&
                followUp.mutate({ user_id: followUpFor.id, status: followUpStatus, note: followUpNote })
              }
            >
              Guardar
            </Button>
          </>
        }
      >
        <div className={styles.followUpForm}>
          <label>¿Qué pasó?</label>
          <Select
            fullWidth
            value={followUpStatus}
            onChange={(v) => setFollowUpStatus(String(v))}
            options={[
              { value: 'contacted', label: 'Ya lo contacté' },
              { value: 'justified', label: 'La inactividad está justificada' },
              { value: 'escalated', label: 'Escalado al manager o a la empresa' },
            ]}
          />
          <label>Nota</label>
          <textarea
            rows={4}
            value={followUpNote}
            onChange={(e) => setFollowUpNote(e.target.value)}
            placeholder="Qué se habló, qué quedó pendiente..."
          />
          {followUp.isError && <p className={styles.formError}>No se pudo guardar la gestión.</p>}
        </div>
      </Modal>
    </div>
  );
}

export default TenantUsage;
