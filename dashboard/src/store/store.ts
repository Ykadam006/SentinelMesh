import { configureStore, createAsyncThunk, createSlice } from '@reduxjs/toolkit';
import { useDispatch, useSelector } from 'react-redux';

export type Severity = 'P1' | 'P2' | 'P3';
export type IncidentStatus = 'ACTIVE' | 'ACKNOWLEDGED' | 'RESOLVED';
export type ServiceStatus = 'HEALTHY' | 'DEGRADED' | 'INCIDENT';

export interface Incident {
  id: string;
  service_id: string;
  title: string;
  description: string;
  status: IncidentStatus;
  severity?: Severity;
  created_at: number;
  acked_at?: number;
  resolved_at?: number;
}

export interface Service {
  service_id: string;
  status: ServiceStatus;
  availability: number;
  error_budget_remaining: number;
  slo_target: number;
  latency_ms: number;
  latency_threshold_ms: number;
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) throw new Error(`${path} responded ${res.status}`);
  return res.json();
}

export const fetchDashboard = createAsyncThunk('ops/fetch', async () => {
  const [incidents, services] = await Promise.all([
    api<Incident[] | null>('/api/incidents'),
    api<Service[] | null>('/api/services'),
  ]);
  return {
    incidents: incidents ?? [],
    services: (services ?? []).sort((a, b) => a.service_id.localeCompare(b.service_id)),
  };
});

export const updateIncident = createAsyncThunk(
  'ops/updateIncident',
  async ({ id, status }: { id: string; status: IncidentStatus }, { dispatch }) => {
    await api('/api/incidents', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ incident_id: id, status }),
    });
    await dispatch(fetchDashboard());
  },
);

interface OpsState {
  incidents: Incident[];
  services: Service[];
  lastUpdated: number | null;
  error: string | null;
  updating: string[];
}

const initialState: OpsState = { incidents: [], services: [], lastUpdated: null, error: null, updating: [] };

const opsSlice = createSlice({
  name: 'ops',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchDashboard.fulfilled, (state, { payload }) => {
        state.incidents = payload.incidents;
        state.services = payload.services;
        state.lastUpdated = Date.now();
        state.error = null;
      })
      .addCase(fetchDashboard.rejected, (state, { error }) => {
        state.error = error.message ?? 'API gateway unreachable';
      })
      .addCase(updateIncident.pending, (state, { meta }) => {
        state.updating.push(meta.arg.id);
      })
      .addCase(updateIncident.fulfilled, (state, { meta }) => {
        state.updating = state.updating.filter((id) => id !== meta.arg.id);
      })
      .addCase(updateIncident.rejected, (state, { meta, error }) => {
        state.updating = state.updating.filter((id) => id !== meta.arg.id);
        state.error = error.message ?? 'Failed to update incident';
      });
  },
});

export const store = configureStore({ reducer: { ops: opsSlice.reducer } });

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
export const useAppDispatch = useDispatch.withTypes<AppDispatch>();
export const useAppSelector = useSelector.withTypes<RootState>();
