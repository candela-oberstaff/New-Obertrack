import api from './client';

export interface UsageOverview {
  eligible_users: number;
  active_users: number;
  never_active: number;
  adoption_rate: number;
  dau: number;
  wau: number;
  mau: number;
  stickiness: number;
  eligible_companies: number;
  active_companies: number;
  company_rate: number;
  avg_active_days: number;

  /** Los mismos números sobre el período inmediatamente anterior. */
  prev_active_users: number;
  prev_adoption_rate: number;
  adoption_delta: number;
  prev_active_companies: number;
  prev_company_rate: number;
  company_delta: number;

  /** Primer día con datos. Null = el contador aún no ha registrado nada. */
  tracking_since: string | null;
  /**
   * Falso mientras el contador no cubra entero el período anterior. Con esto en
   * falso NO se pintan flechas: la "caída" que saldría sería el hueco de datos.
   */
  comparable: boolean;
}

export interface ModuleUsage {
  module: string;
  users: number;
  hits: number;
  rate: number;
  prev_users: number;
  prev_rate: number;
  /** Variación en PUNTOS porcentuales, no en porcentaje de variación. */
  delta: number;
}

export interface UsageDay {
  day: string;
  users: number;
}

export interface UsageSummary {
  overview: UsageOverview;
  modules: ModuleUsage[];
  trend: UsageDay[];
  online: number;
  days: number;
}

export interface CompanyUsage {
  company_id: number;
  company_name: string;
  total_users: number;
  active_users: number;
  rate: number;
  chat_users: number;
  chat_rate: number;
  hits: number;
  last_active: string | null;
  prev_active_users: number;
  prev_rate: number;
  delta: number;
}

export interface PersonUsage {
  user_id: number;
  name: string;
  email: string;
  user_type: string;
  company_id: number;
  company_name: string;
  active_days: number;
  hits: number;
  last_active: string | null;
  modules: string;
  online: boolean;
}

export interface NeverActiveUser {
  user_id: number;
  name: string;
  email: string;
  user_type: string;
  company_id: number;
  company_name: string;
  created_at: string;
  days_since: number;
  /** true = el alta es posterior al inicio de la medición, así que es un hecho. */
  certain: boolean;
}

export interface PeopleFilters {
  days: number;
  scope?: 'clients' | 'all';
  companyId?: number;
  q?: string;
  status?: '' | 'active' | 'inactive';
  page?: number;
  limit?: number;
}

export type UsageBoard = 'companies' | 'people' | 'activation';

const scopeParam = (scope?: 'clients' | 'all') => (scope === 'all' ? { scope: 'all' } : {});

export const usageService = {
  /** companyId acota el mismo resumen a UNA empresa: es lo que pinta su ficha. */
  getSummary: async (days: number, scope?: 'clients' | 'all', companyId?: number): Promise<UsageSummary> => {
    const { data } = await api.get('/metrics/usage', {
      params: { days, ...scopeParam(scope), ...(companyId ? { company_id: companyId } : {}) },
    });
    return data;
  },

  getCompanies: async (days: number): Promise<CompanyUsage[]> => {
    const { data } = await api.get('/metrics/usage/companies', { params: { days } });
    return data.data ?? [];
  },

  getPeople: async (f: PeopleFilters): Promise<{ data: PersonUsage[]; total: number }> => {
    const { data } = await api.get('/metrics/usage/people', {
      params: {
        days: f.days,
        ...scopeParam(f.scope),
        ...(f.companyId ? { company_id: f.companyId } : {}),
        ...(f.q ? { q: f.q } : {}),
        ...(f.status ? { status: f.status } : {}),
        page: f.page ?? 1,
        limit: f.limit ?? 50,
      },
    });
    return { data: data.data ?? [], total: data.total ?? 0 };
  },

  getActivation: async (
    scope?: 'clients' | 'all',
    page = 1,
    limit = 25,
    companyId?: number,
  ): Promise<{ data: NeverActiveUser[]; total: number }> => {
    const { data } = await api.get('/metrics/usage/activation', {
      params: { ...scopeParam(scope), page, limit, ...(companyId ? { company_id: companyId } : {}) },
    });
    return { data: data.data ?? [], total: data.total ?? 0 };
  },

  /**
   * Quién tiene la app abierta AHORA. Va aparte del resto de métricas porque es
   * el único dato en vivo: se repregunta cada pocos segundos sin recalcular los
   * agregados del período, que son caros y no cambian en ese rato.
   */
  getOnline: async (): Promise<number[]> => {
    const { data } = await api.get('/metrics/usage/online');
    return data.user_ids ?? [];
  },

  /**
   * Descarga la tabla como Excel. Va por axios y no por un <a href> para que
   * pase por el interceptor que renueva la sesión: con la cookie recién
   * vencida, un enlace directo bajaría la página de login como si fuera la hoja.
   */
  exportBoard: async (board: UsageBoard, f: PeopleFilters): Promise<void> => {
    const response = await api.get('/metrics/usage/export', {
      responseType: 'blob',
      params: {
        board,
        days: f.days,
        ...scopeParam(f.scope),
        ...(f.companyId ? { company_id: f.companyId } : {}),
        ...(f.q ? { q: f.q } : {}),
        ...(f.status ? { status: f.status } : {}),
      },
    });
    const url = URL.createObjectURL(response.data as Blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `uso_${board}_${new Date().toISOString().slice(0, 10)}.xlsx`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  },
};
