

interface CompanyMetric {
  id: number
  name: string
  professionals: number
  hours_this_month: number
  tasks_completed: number
  active_users: number
}

interface CompanyTableProps {
  companies: CompanyMetric[]
}

export function CompanyTable({ companies }: CompanyTableProps) {
  return (
    <div className="admin-content">
      <div className="admin-section-header">
        <h3>Empresas Activas</h3>
      </div>
      <div className="companies-table">
        <table>
          <thead>
            <tr>
              <th>Empresa</th>
              <th>Profesionales</th>
              <th>Usuarios Activos</th>
              <th>Horas Mes</th>
              <th>Tareas Completadas</th>
            </tr>
          </thead>
          <tbody>
            {companies.map(company => (
              <tr key={company.id}>
                <td className="company-name">{company.name || 'Sin nombre'}</td>
                <td>{company.professionals}</td>
                <td>{company.active_users}</td>
                <td>{company.hours_this_month.toFixed(1)}h</td>
                <td>{company.tasks_completed}</td>
              </tr>
            ))}
            {companies.length === 0 && (
              <tr>
                <td colSpan={5} className="no-data">No hay empresas registradas</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
