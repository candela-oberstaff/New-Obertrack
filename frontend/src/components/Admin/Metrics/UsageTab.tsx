import React, { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import {
  Activity,
  Building2,
  Download,
  Radio,
  Repeat,
  Search,
  TrendingUp,
  UserPlus,
  Users,
} from 'lucide-react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  usageService,
  type CompanyUsage,
  type NeverActiveUser,
  type PersonUsage,
  type UsageBoard,
} from '../../../services/usage.service';
import { adminService } from '../../../services/admin.service';
import { healthSignal, HEALTH_COLOR } from '../../../pages/Tenants/accountHealth';
import { Delta, healthOf, ModuleBars, moduleLabel, Pager, trackedDaysFrom, UsageNotice } from './usageShared';
import { Select } from '../../ui/Select';
import { Button } from '../../ui/Button';
import { Modal } from '../../ui/Modal';
import styles from './UsageTab.module.css';

interface UsageTabProps {
  days: number;
}

const UsageTab: React.FC<UsageTabProps> = ({ days }) => {
  const navigate = useNavigate();
  const [scope, setScope] = useState<'clients' | 'all'>('clients');
  const [board, setBoard] = useState<UsageBoard>('companies');
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<'' | 'active' | 'inactive'>('');
  const [companyFilter, setCompanyFilter] = useState(0);
  const [page, setPage] = useState(1);
  const [exporting, setExporting] = useState(false);
  const [followUpFor, setFollowUpFor] = useState<{ id: number; name: string } | null>(null);
  const [followUpStatus, setFollowUpStatus] = useState('contacted');
  const [followUpNote, setFollowUpNote] = useState('');

  const { data: summary, isLoading } = useQuery({
    queryKey: ['usage-summary', days, scope],
    queryFn: () => usageService.getSummary(days, scope),
  });

  const { data: companies = [] } = useQuery({
    queryKey: ['usage-companies', days],
    queryFn: () => usageService.getCompanies(days),
  });

  const { data: people } = useQuery({
    queryKey: ['usage-people', days, scope, search, status, companyFilter, page],
    queryFn: () =>
      usageService.getPeople({ days, scope, q: search, status, companyId: companyFilter, page, limit: 25 }),
    enabled: board === 'people',
    placeholderData: (prev) => prev,
  });

  const { data: activation } = useQuery({
    queryKey: ['usage-activation', scope, page],
    queryFn: () => usageService.getActivation(scope, page, 25),
    enabled: board === 'activation',
    placeholderData: (prev) => prev,
  });

  // La presencia se repregunta sola cada 20 s. Lo demás no: recalcular los
  // agregados del período cada veinte segundos sería caro y no cambiaría nada.
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

  const trend = (summary?.trend ?? []).map((d) => ({ name: d.day.slice(5), users: d.users }));

  const companyOptions = useMemo(
    () => [
      { value: 0, label: 'Todas las empresas' },
      ...companies.map((c: CompanyUsage) => ({ value: c.company_id, label: c.company_name })),
    ],
    [companies],
  );

  const switchBoard = (next: UsageBoard) => {
    setBoard(next);
    setPage(1);
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      await usageService.exportBoard(board, { days, scope, q: search, status, companyId: companyFilter });
    } finally {
      setExporting(false);
    }
  };

  if (isLoading && !summary) {
    return (
      <div className={styles.loading}>
        <Activity size={40} className={styles.spin} />
        <p>Calculando el uso de la app...</p>
      </div>
    );
  }

  const trackingSince = overview?.tracking_since ? new Date(overview.tracking_since) : null;
  const trackedDays = trackedDaysFrom(trackingSince);

  const activationTotal = activation?.total ?? 0;

  return (
    <div className={styles.wrap}>
      <UsageNotice
        trackingSince={trackingSince}
        trackedDays={trackedDays}
        days={days}
        comparable={comparable}
      />

      <div className={styles.scopeRow}>
        <div className={styles.scopeToggle}>
          <button className={scope === 'clients' ? styles.active : ''} onClick={() => setScope('clients')}>
            Solo clientes
          </button>
          <button className={scope === 'all' ? styles.active : ''} onClick={() => setScope('all')}>
            Incluir equipo interno
          </button>
        </div>
        <span className={styles.scopeHint}>
          {scope === 'clients'
            ? 'Superadmins, Customer Success y analistas quedan fuera del cálculo.'
            : 'Se cuenta a todo el mundo, incluido el equipo de Obertrack.'}
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
              {(overview?.adoption_rate ?? 0).toFixed(1)}%
              <Delta value={overview?.adoption_delta ?? 0} comparable={comparable} />
            </h3>
            <small>
              {overview?.active_users ?? 0} de {overview?.eligible_users ?? 0} personas la abrieron
            </small>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#dbeafe', color: '#1d4ed8' }}>
            <Building2 size={24} />
          </div>
          <div className={styles.statInfo}>
            <label>Empresas que la usan</label>
            <h3>
              {(overview?.company_rate ?? 0).toFixed(1)}%
              <Delta value={overview?.company_delta ?? 0} comparable={comparable} />
            </h3>
            <small>
              {overview?.active_companies ?? 0} de {overview?.eligible_companies ?? 0} empresas
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
              {onlineIds.length}
            </h3>
            <small>Con la app abierta en este momento</small>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#fef9c3', color: '#a16207' }}>
            <Repeat size={24} />
          </div>
          <div className={styles.statInfo}>
            <label>Constancia</label>
            <h3>{(overview?.stickiness ?? 0).toFixed(0)}%</h3>
            <small>
              De quien entra al mes, cuántos entran hoy ({overview?.dau ?? 0} de {overview?.mau ?? 0})
            </small>
          </div>
        </div>
      </div>

      <div className={styles.chartsGrid}>
        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <h4>Personas activas por día</h4>
            <p>Cuánta gente distinta abrió la app cada día</p>
          </div>
          <ResponsiveContainer width="100%" height={280}>
            <AreaChart data={trend}>
              <defs>
                <linearGradient id="usageFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
              <XAxis dataKey="name" tick={{ fontSize: 11, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <YAxis allowDecimals={false} tick={{ fontSize: 11, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <Tooltip formatter={(v: any) => [`${v} personas`, 'Activas']} />
              <Area type="monotone" dataKey="users" stroke="#8b5cf6" strokeWidth={2} fill="url(#usageFill)" />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <h4>% de uso por módulo</h4>
            <p>Gente distinta que lo tocó, sobre el total de personas</p>
          </div>
          <ModuleBars modules={summary?.modules ?? []} comparable={comparable} />
        </div>
      </div>

      <div className={styles.card}>
        <div className={styles.boardHeader}>
          <div className={styles.scopeToggle}>
            <button className={board === 'companies' ? styles.active : ''} onClick={() => switchBoard('companies')}>
              <Building2 size={15} /> Empresas
            </button>
            <button className={board === 'people' ? styles.active : ''} onClick={() => switchBoard('people')}>
              <Users size={15} /> Personas
            </button>
            <button className={board === 'activation' ? styles.active : ''} onClick={() => switchBoard('activation')}>
              <UserPlus size={15} /> Sin estrenar
              {activationTotal > 0 && <span className={styles.tabCount}>{activationTotal}</span>}
            </button>
          </div>

          <div className={styles.filters}>
            {board === 'people' && (
              <>
                <div className={styles.searchBox}>
                  <Search size={15} />
                  <input
                    value={search}
                    onChange={(e) => { setSearch(e.target.value); setPage(1); }}
                    placeholder="Buscar por nombre o correo"
                  />
                </div>
                <Select
                  value={companyFilter}
                  onChange={(v) => { setCompanyFilter(Number(v)); setPage(1); }}
                  options={companyOptions}
                />
                <Select
                  value={status}
                  onChange={(v) => { setStatus(v as '' | 'active' | 'inactive'); setPage(1); }}
                  options={[
                    { value: '', label: 'Todas' },
                    { value: 'active', label: 'Con actividad' },
                    { value: 'inactive', label: 'Sin actividad' },
                  ]}
                />
              </>
            )}
            <Button variant="secondary" size="sm" leftIcon={<Download size={15} />} loading={exporting} onClick={handleExport}>
              Excel
            </Button>
          </div>
        </div>

        {board === 'companies' && (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Empresa</th>
                  <th>Personas</th>
                  <th>Activas</th>
                  <th>% de uso</th>
                  <th>% usa el chat</th>
                  <th>Última señal</th>
                  <th>Estado</th>
                </tr>
              </thead>
              <tbody>
                {companies.length === 0 && (
                  <tr>
                    <td colSpan={7} className={styles.empty}>Sin empresas que mostrar.</td>
                  </tr>
                )}
                {companies.map((c: CompanyUsage) => {
                  const health = healthOf(c.rate, c.active_users);
                  const signal = healthSignal(c.last_active);
                  return (
                    <tr
                      key={c.company_id}
                      className={styles.clickable}
                      onClick={() => navigate(`/admin/tenants/${c.company_id}`)}
                      title="Abrir la ficha de la empresa"
                    >
                      <td className={styles.strong}>{c.company_name}</td>
                      <td>{c.total_users}</td>
                      <td>{c.active_users}</td>
                      <td>
                        <div className={styles.inlineBar}>
                          <div className={styles.inlineBarTrack}>
                            <div className={styles.inlineBarFill} style={{ width: `${Math.min(100, c.rate)}%` }} />
                          </div>
                          <span>{c.rate.toFixed(0)}%</span>
                          <Delta value={c.delta} comparable={comparable} />
                        </div>
                      </td>
                      <td>{c.chat_rate.toFixed(0)}%</td>
                      {/* Mismo criterio y mismos colores que el semáforo de la
                          ficha de empresa: dos pantallas que dicen "hace 40
                          días" no pueden discrepar en si eso es grave. */}
                      <td style={{ color: HEALTH_COLOR[signal.level], fontWeight: 600 }}>{signal.label}</td>
                      <td>
                        <span className={`${styles.badge} ${health.cls}`}>{health.label}</span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {board === 'people' && (
          <>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Persona</th>
                    <th>Empresa</th>
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
                      <td colSpan={7} className={styles.empty}>Nadie coincide con el filtro.</td>
                    </tr>
                  )}
                  {(people?.data ?? []).map((p: PersonUsage) => {
                    // La lista se refresca con el filtro; la presencia, cada 20 s.
                    // Se pinta el dato en vivo, que es el que el usuario mira.
                    const online = onlineSet.has(p.user_id);
                    const signal = healthSignal(p.last_active);
                    return (
                      <tr
                        key={p.user_id}
                        className={styles.clickable}
                        onClick={() => navigate(`/admin/users/${p.user_id}`)}
                        title="Abrir la ficha de la persona"
                      >
                        <td>
                          <div className={styles.person}>
                            <span className={styles.strong}>{p.name}</span>
                            <small>{p.email}</small>
                          </div>
                        </td>
                        <td className={styles.muted}>{p.company_name || '—'}</td>
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
        )}

        {board === 'activation' && (
          <>
            <p className={styles.boardIntro}>
              Cuentas activas que <strong>nunca</strong> han aparecido en el contador de uso: se les dio
              acceso y no lo estrenaron. Las marcadas como <em>confirmado</em> se dieron de alta después
              de que empezáramos a medir, así que son un hecho; en las demás solo sabemos que no han
              entrado desde entonces.
            </p>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Persona</th>
                    <th>Empresa</th>
                    <th>Alta</th>
                    <th>Días sin estrenar</th>
                    <th>Certeza</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {(activation?.data ?? []).length === 0 && (
                    <tr>
                      <td colSpan={6} className={styles.empty}>
                        Todo el mundo ha entrado alguna vez. Buena señal.
                      </td>
                    </tr>
                  )}
                  {(activation?.data ?? []).map((u: NeverActiveUser) => (
                    <tr
                      key={u.user_id}
                      className={styles.clickable}
                      onClick={() => navigate(`/admin/users/${u.user_id}`)}
                      title="Abrir la ficha de la persona"
                    >
                      <td>
                        <div className={styles.person}>
                          <span className={styles.strong}>{u.name}</span>
                          <small>{u.email}</small>
                        </div>
                      </td>
                      <td className={styles.muted}>{u.company_name || '—'}</td>
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

      <div className={styles.footNote}>
        <TrendingUp size={14} />
        <span>
          Se cuenta una persona como activa en un módulo cuando entra a esa pantalla, aunque solo mire.
          Los sondeos que el menú lanza solo por tener la app abierta (mensajes sin leer, campanita,
          novedades) no cuentan como uso del módulo.
        </span>
      </div>

      {/* La gestión se anota SIN salir del panel: obligar a abrir la ficha en
          otra pestaña para escribir dos líneas es lo que hace que la bitácora
          se quede vacía. Va a la misma tabla de seguimientos que ya usa CS. */}
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
};

export default UsageTab;
