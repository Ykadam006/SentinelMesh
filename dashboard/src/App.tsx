import { useEffect, useState } from 'react';
import { Provider } from 'react-redux';
import {
  Activity, AlertTriangle, ArrowDown, CheckCircle2, Clock, ExternalLink, Gauge, Server, ShieldAlert, Timer, WifiOff,
} from 'lucide-react';
import { store, fetchDashboard, updateIncident, useAppDispatch, useAppSelector } from './store/store';
import type { Incident, IncidentStatus, Service, ServiceStatus, Severity } from './store/store';

const POLL_MS = 5000;

type Tone = 'ok' | 'warn' | 'crit' | 'info' | 'neutral';

const serviceTone: Record<ServiceStatus, Tone> = { HEALTHY: 'ok', DEGRADED: 'warn', INCIDENT: 'crit' };
const serviceLabel: Record<ServiceStatus, string> = { HEALTHY: 'Healthy', DEGRADED: 'Degraded', INCIDENT: 'Incident' };
const statusTone: Record<IncidentStatus, Tone> = { ACTIVE: 'crit', ACKNOWLEDGED: 'info', RESOLVED: 'ok' };
const statusLabel: Record<IncidentStatus, string> = { ACTIVE: 'Active', ACKNOWLEDGED: 'Acknowledged', RESOLVED: 'Resolved' };
const severityTone: Record<Severity, Tone> = { P1: 'crit', P2: 'warn', P3: 'neutral' };
const severityRank: Record<Severity, number> = { P1: 0, P2: 1, P3: 2 };

const budgetTone = (pct: number): Tone => (pct > 50 ? 'ok' : pct > 20 ? 'warn' : 'crit');
const humanize = (title: string) => title.replace(/([a-z])([A-Z])/g, '$1 $2');
const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`;

function duration(seconds: number) {
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${m % 60}m`;
  return `${Math.floor(h / 24)}d ${h % 24}h`;
}

function useNow(intervalMs = 1000) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(t);
  }, [intervalMs]);
  return now;
}

function Pill({ tone, children }: { tone: Tone; children: React.ReactNode }) {
  return (
    <span className="pill" data-tone={tone}>
      <span className="pill__dot" aria-hidden="true" />
      {children}
    </span>
  );
}

function Header({ now }: { now: number }) {
  const { lastUpdated, error } = useAppSelector((s) => s.ops);
  return (
    <header className="topbar glass">
      <div className="topbar__inner">
        <div className="brand">
          <span className="brand__mark" aria-hidden="true"><ShieldAlert size={20} /></span>
          <span>SentinelMesh</span>
          <span className="chip">local</span>
        </div>
        <p className={`live${error ? ' live--down' : ''}`}>
          {error ? (
            <><WifiOff size={16} aria-hidden="true" /> Gateway unreachable</>
          ) : (
            <>
              <span className="live__dot" aria-hidden="true" />
              {lastUpdated ? `Live · updated ${duration((now - lastUpdated) / 1000)} ago` : 'Connecting…'}
            </>
          )}
        </p>
      </div>
    </header>
  );
}

function Overview({ now }: { now: number }) {
  const { incidents, services, lastUpdated } = useAppSelector((s) => s.ops);
  const open = incidents
    .filter((i) => i.status !== 'RESOLVED')
    .sort((a, b) => severityRank[a.severity ?? 'P3'] - severityRank[b.severity ?? 'P3'] || b.created_at - a.created_at);
  const active = open.filter((i) => i.status === 'ACTIVE');
  const top = open[0];
  const tone: Tone = active.length ? 'crit' : open.length ? 'warn' : 'ok';

  const healthy = services.filter((s) => s.status === 'HEALTHY');
  const unhealthy = services.filter((s) => s.status !== 'HEALTHY');
  const lowest = services.reduce<Service | undefined>(
    (min, s) => (!min || s.error_budget_remaining < min.error_budget_remaining ? s : min), undefined);
  const resolved = incidents.filter((i) => i.status === 'RESOLVED' && i.resolved_at);
  const mttr = resolved.length
    ? resolved.reduce((sum, i) => sum + (i.resolved_at! - i.created_at), 0) / resolved.length
    : null;

  const headline = !lastUpdated
    ? 'Connecting to SentinelMesh…'
    : active.length
      ? plural(active.length, 'active incident')
      : open.length
        ? `${plural(open.length, 'incident')} acknowledged`
        : 'All systems operational';

  return (
    <section className="hero glass" data-tone={tone} aria-labelledby="overview-title">
      <div className="hero__head">
        <div>
          <p className="eyebrow">
            {tone === 'ok' ? <CheckCircle2 size={16} aria-hidden="true" /> : <AlertTriangle size={16} aria-hidden="true" />}
            {tone === 'ok' ? 'Operational' : tone === 'crit' ? 'Needs attention' : 'Being handled'}
          </p>
          <h1 id="overview-title">{headline}</h1>
          <p className="hero__sub">
            {top
              ? <>{top.severity ?? 'P3'} · <span className="mono">{top.service_id}</span> · {humanize(top.title)} · opened {duration(now / 1000 - top.created_at)} ago</>
              : `Monitoring ${plural(services.length, 'service')} against their SLO targets. No open incidents.`}
          </p>
        </div>
        {open.length > 0 && (
          <a className="btn btn--primary" href="#incidents">
            Review incidents <ArrowDown size={16} aria-hidden="true" />
          </a>
        )}
      </div>

      <div className="kpis">
        <div className="kpi">
          <p className="kpi__label"><AlertTriangle size={16} aria-hidden="true" /> Active incidents</p>
          <p className={`kpi__value${active.length ? ' tone-crit' : ''}`}>{active.length}</p>
          <p className="kpi__foot">
            {open.filter((i) => i.severity === 'P1').length} P1 · {open.length - active.length} acknowledged
          </p>
        </div>
        <div className="kpi">
          <p className="kpi__label"><Server size={16} aria-hidden="true" /> Services healthy</p>
          <p className="kpi__value">
            {healthy.length}<span className="kpi__unit">/{services.length}</span>
          </p>
          <p className="kpi__foot">
            {unhealthy.length ? unhealthy.map((s) => s.service_id).join(', ') : 'All within SLO'}
          </p>
        </div>
        <div className="kpi">
          <p className="kpi__label"><Gauge size={16} aria-hidden="true" /> Lowest error budget</p>
          <p className={`kpi__value tone-${lowest ? budgetTone(lowest.error_budget_remaining) : 'neutral'}`}>
            {lowest ? Math.round(lowest.error_budget_remaining) : '—'}<span className="kpi__unit">%</span>
          </p>
          <p className="kpi__foot">
            {!lowest ? 'No services reporting' : lowest.error_budget_remaining >= 100 ? 'No budget burned yet' : lowest.service_id}
          </p>
        </div>
        <div className="kpi">
          <p className="kpi__label"><Timer size={16} aria-hidden="true" /> Mean time to resolve</p>
          <p className="kpi__value">{mttr === null ? '—' : duration(mttr)}</p>
          <p className="kpi__foot">Across {plural(resolved.length, 'resolved incident')}</p>
        </div>
      </div>
    </section>
  );
}

function ServiceCard({ s }: { s: Service }) {
  const tone = serviceTone[s.status];
  const budget = Math.round(s.error_budget_remaining);
  const bTone = budgetTone(budget);
  return (
    <article className="svc glass" data-tone={tone}>
      <header className="svc__head">
        <h3 className="svc__name">{s.service_id}</h3>
        <Pill tone={tone}>{serviceLabel[s.status]}</Pill>
      </header>
      <div>
        <span className="svc__big">{s.availability.toFixed(2)}<small>%</small></span>
        <span className="svc__caption">Availability · SLO {s.slo_target}%</span>
      </div>
      <div>
        <div className="meter-label">
          <span>Error budget</span>
          <span className={`tone-${bTone}`}>{budget}% left</span>
        </div>
        <div
          className="meter" role="meter" aria-label={`${s.service_id} error budget remaining`}
          aria-valuemin={0} aria-valuemax={100} aria-valuenow={budget}
        >
          <span className={`meter__fill bg-${bTone}`} style={{ width: `${budget}%` }} />
        </div>
      </div>
      <dl className="svc__stats">
        <div>
          <dt>Latency</dt>
          <dd className={s.latency_ms > s.latency_threshold_ms ? 'tone-crit' : undefined}>
            {s.latency_ms ? `${Math.round(s.latency_ms)} ms` : '—'}
          </dd>
        </div>
        <div>
          <dt>Threshold</dt>
          <dd>{s.latency_threshold_ms} ms</dd>
        </div>
      </dl>
    </article>
  );
}

function Services() {
  const { services, lastUpdated } = useAppSelector((s) => s.ops);
  return (
    <section aria-labelledby="services-title">
      <div className="section-head">
        <h2 id="services-title"><Activity size={20} aria-hidden="true" /> Services</h2>
        <p>Availability and error budget against each service's SLO</p>
      </div>
      <div className="svc-grid">
        {!lastUpdated && !services.length
          ? Array.from({ length: 5 }, (_, i) => <div key={i} className="svc glass skeleton" aria-hidden="true" />)
          : services.map((s) => <ServiceCard key={s.service_id} s={s} />)}
      </div>
    </section>
  );
}

function IncidentRow({ inc, now }: { inc: Incident; now: number }) {
  const dispatch = useAppDispatch();
  const busy = useAppSelector((s) => s.ops.updating.includes(inc.id));
  const severity = inc.severity ?? 'P3';
  const label = `${humanize(inc.title)} on ${inc.service_id}`;
  const update = (status: IncidentStatus) => dispatch(updateIncident({ id: inc.id, status }));

  return (
    <li className="inc glass" data-tone={statusTone[inc.status]}>
      <div>
        <div className="inc__meta">
          <Pill tone={severityTone[severity]}>{severity}</Pill>
          <Pill tone={statusTone[inc.status]}>{statusLabel[inc.status]}</Pill>
          <span className="mono">{inc.service_id}</span>
          <span>· {duration(now / 1000 - inc.created_at)} ago</span>
        </div>
        <h3 className="inc__title">{humanize(inc.title)}</h3>
        <p className="inc__desc">{inc.description}</p>
        <p className="inc__timing">
          <Clock size={14} aria-hidden="true" />
          {inc.status === 'RESOLVED' && inc.resolved_at
            ? `Resolved in ${duration(inc.resolved_at - inc.created_at)}`
            : `Open for ${duration(now / 1000 - inc.created_at)}`}
          {inc.acked_at ? ` · acknowledged after ${duration(inc.acked_at - inc.created_at)}` : ''}
        </p>
      </div>
      {inc.status !== 'RESOLVED' && (
        <div className="inc__actions">
          {inc.status === 'ACTIVE' && (
            <button className="btn btn--ghost" disabled={busy} aria-label={`Acknowledge ${label}`} onClick={() => update('ACKNOWLEDGED')}>
              Acknowledge
            </button>
          )}
          <button className="btn btn--secondary" disabled={busy} aria-label={`Resolve ${label}`} onClick={() => update('RESOLVED')}>
            <CheckCircle2 size={16} aria-hidden="true" /> Resolve
          </button>
        </div>
      )}
    </li>
  );
}

const FILTERS = [
  { key: 'open', label: 'Open', match: (i: Incident) => i.status !== 'RESOLVED' },
  { key: 'resolved', label: 'Resolved', match: (i: Incident) => i.status === 'RESOLVED' },
  { key: 'all', label: 'All', match: () => true },
] as const;

function Incidents({ now }: { now: number }) {
  const incidents = useAppSelector((s) => s.ops.incidents);
  const [filter, setFilter] = useState<(typeof FILTERS)[number]['key']>('open');
  const current = FILTERS.find((f) => f.key === filter)!;
  const list = incidents.filter(current.match);

  return (
    <section id="incidents" aria-labelledby="incidents-title">
      <div className="section-head">
        <h2 id="incidents-title"><AlertTriangle size={20} aria-hidden="true" /> Incidents</h2>
        <div className="segmented" role="group" aria-label="Filter incidents">
          {FILTERS.map((f) => (
            <button key={f.key} type="button" aria-pressed={filter === f.key} onClick={() => setFilter(f.key)}>
              {f.label}
              <span className="count">{incidents.filter(f.match).length}</span>
            </button>
          ))}
        </div>
      </div>
      {list.length ? (
        <ul className="inc-list">
          {list.map((inc) => <IncidentRow key={inc.id} inc={inc} now={now} />)}
        </ul>
      ) : (
        <div className="empty glass">
          <span className="empty__icon" aria-hidden="true"><CheckCircle2 size={28} /></span>
          <p className="empty__title">{filter === 'open' ? 'No open incidents' : 'Nothing here yet'}</p>
          <p>{filter === 'open' ? 'Every service is within its alert thresholds.' : 'Incidents will appear once alerts fire.'}</p>
        </div>
      )}
    </section>
  );
}

function Footer() {
  const host = window.location.hostname;
  const tools = [['Grafana', 3000], ['Prometheus', 9090], ['Kibana', 5601]] as const;
  return (
    <footer className="footer">
      <p>Refreshes every {POLL_MS / 1000}s · SLOs evaluated by the alert engine</p>
      <nav aria-label="Observability tools">
        {tools.map(([name, port]) => (
          <a key={name} className="btn btn--ghost" href={`http://${host}:${port}`} target="_blank" rel="noreferrer">
            {name} <ExternalLink size={14} aria-hidden="true" />
          </a>
        ))}
      </nav>
    </footer>
  );
}

function Dashboard() {
  const dispatch = useAppDispatch();
  const { error, lastUpdated, incidents } = useAppSelector((s) => s.ops);
  const now = useNow();
  const active = incidents.filter((i) => i.status === 'ACTIVE').length;

  useEffect(() => {
    const load = () => dispatch(fetchDashboard());
    load();
    const t = setInterval(load, POLL_MS);
    return () => clearInterval(t);
  }, [dispatch]);

  useEffect(() => {
    document.title = active ? `(${active}) SentinelMesh` : 'SentinelMesh';
  }, [active]);

  return (
    <>
      <div className="backdrop" data-tone={active ? 'crit' : 'ok'} aria-hidden="true">
        <span className="orb orb--1" />
        <span className="orb orb--2" />
        <span className="orb orb--3" />
      </div>
      <Header now={now} />
      <main className="page">
        {error && (
          <div className="alert" role="alert">
            <WifiOff size={18} aria-hidden="true" />
            Can't reach the API gateway{lastUpdated ? ', showing last known data' : ''}. Retrying every {POLL_MS / 1000}s.
          </div>
        )}
        <Overview now={now} />
        <Services />
        <Incidents now={now} />
        <Footer />
      </main>
    </>
  );
}

export default function App() {
  return (
    <Provider store={store}>
      <Dashboard />
    </Provider>
  );
}
