import { useEffect } from 'react';
import { Provider, useDispatch, useSelector } from 'react-redux';
import { Activity, AlertTriangle, CheckCircle2, Server, ShieldAlert } from 'lucide-react';
import { store, setIncidents } from './store/store';
import type { RootState, Incident } from './store/store';
import './index.css';

const DashboardContent = () => {
  const dispatch = useDispatch();
  const incidents = useSelector((state: RootState) => state.incidents.items);

  useEffect(() => {
    // Poll for incidents
    const fetchIncidents = async () => {
      try {
        const res = await fetch('http://localhost:8080/api/incidents');
        if (res.ok) {
          const data = await res.json();
          dispatch(setIncidents(data || []));
        }
      } catch (err) {
        console.error("Failed to fetch incidents", err);
      }
    };

    fetchIncidents();
    const interval = setInterval(fetchIncidents, 5000);
    return () => clearInterval(interval);
  }, [dispatch]);

  const activeIncidents = incidents.filter(i => i.status === 'ACTIVE');
  const systemStatus = activeIncidents.length > 0 ? 'Degraded' : 'Operational';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <header className="header-blur" style={{ padding: '20px 40px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', position: 'sticky', top: 0, zIndex: 100 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <ShieldAlert size={32} color="var(--accent)" />
          <h1 className="glow-text" style={{ fontSize: '1.5rem', fontWeight: 700, margin: 0 }}>SentinelMesh</h1>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <span style={{ color: 'var(--text-secondary)' }}>System Status:</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: systemStatus === 'Operational' ? 'var(--success)' : 'var(--danger)' }}>
            {systemStatus === 'Operational' ? <CheckCircle2 size={20} /> : <AlertTriangle size={20} className="animate-pulse" />}
            <span style={{ fontWeight: 600 }}>{systemStatus}</span>
          </div>
        </div>
      </header>

      <main style={{ padding: '40px', flex: 1, display: 'flex', flexDirection: 'column', gap: '32px', maxWidth: '1400px', margin: '0 auto', width: '100%' }}>
        
        {/* Top KPI Cards */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '24px' }}>
          <div className="glass-panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', color: 'var(--text-secondary)', marginBottom: '16px' }}>
              <span style={{ fontWeight: 600 }}>Active Incidents</span>
              <AlertTriangle size={20} color={activeIncidents.length > 0 ? "var(--danger)" : "var(--text-secondary)"} />
            </div>
            <div className={`stat-value ${activeIncidents.length > 0 ? 'glow-text-danger' : ''}`} style={{ color: activeIncidents.length > 0 ? 'var(--danger)' : 'var(--text-primary)' }}>
              {activeIncidents.length}
            </div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>Requires immediate attention</div>
          </div>

          <div className="glass-panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', color: 'var(--text-secondary)', marginBottom: '16px' }}>
              <span style={{ fontWeight: 600 }}>Services Monitored</span>
              <Server size={20} color="var(--accent)" />
            </div>
            <div className="stat-value">4</div>
            <div style={{ color: 'var(--success)', fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <CheckCircle2 size={16} /> All systems nominal
            </div>
          </div>

          <div className="glass-panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', color: 'var(--text-secondary)', marginBottom: '16px' }}>
              <span style={{ fontWeight: 600 }}>Global P95 Latency</span>
              <Activity size={20} color="var(--warning)" />
            </div>
            <div className="stat-value">142<span style={{ fontSize: '1.5rem', color: 'var(--text-secondary)' }}>ms</span></div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>Averaged across cluster</div>
          </div>
        </div>

        {/* Incidents List */}
        <div className="glass-panel" style={{ flex: 1 }}>
          <h2 style={{ marginBottom: '24px', fontSize: '1.25rem', display: 'flex', alignItems: 'center', gap: '10px' }}>
            <AlertTriangle size={24} color="var(--accent)" />
            Recent Incidents
          </h2>
          
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {incidents.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '40px', color: 'var(--text-secondary)' }}>
                <CheckCircle2 size={48} color="var(--success)" style={{ margin: '0 auto 16px', opacity: 0.5 }} />
                <p>No incidents reported. All services are operating normally.</p>
              </div>
            ) : (
              incidents.map((incident: Incident) => (
                <div key={incident.id} style={{ 
                  background: 'rgba(255, 255, 255, 0.03)', 
                  border: '1px solid var(--glass-border)',
                  borderRadius: '12px',
                  padding: '20px',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  borderLeft: incident.status === 'ACTIVE' ? '4px solid var(--danger)' : '4px solid var(--success)'
                }}>
                  <div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '8px' }}>
                      <span className={`badge ${incident.status === 'ACTIVE' ? 'active animate-pulse' : 'resolved'}`}>
                        {incident.status}
                      </span>
                      <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{incident.service_id}</span>
                      <span style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
                        {new Date(incident.created_at * 1000).toLocaleString()}
                      </span>
                    </div>
                    <h3 style={{ fontSize: '1.1rem', marginBottom: '4px' }}>{incident.title}</h3>
                    <p style={{ color: 'var(--text-secondary)', fontSize: '0.95rem' }}>{incident.description}</p>
                  </div>
                  {incident.status === 'ACTIVE' && (
                    <button className="btn" onClick={() => resolveIncident(incident.id)}>
                      Acknowledge & Resolve
                    </button>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      </main>
    </div>
  );
};

const resolveIncident = async (id: string) => {
  try {
    // We would ideally use Redux Thunk here, keeping it simple for the demo
    await fetch('http://localhost:8080/api/incidents', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ incident_id: id, status: 'RESOLVED' })
    });
  } catch (err) {
    console.error(err);
  }
};

const App = () => {
  return (
    <Provider store={store}>
      <DashboardContent />
    </Provider>
  );
};

export default App;
