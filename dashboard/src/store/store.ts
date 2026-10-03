import { configureStore, createSlice } from '@reduxjs/toolkit';
import type { PayloadAction } from '@reduxjs/toolkit';

export interface Incident {
  id: string;
  service_id: string;
  title: string;
  description: string;
  status: string;
  created_at: number;
  resolved_at: number;
}

interface IncidentsState {
  items: Incident[];
  loading: boolean;
}

const initialState: IncidentsState = {
  items: [],
  loading: false,
};

const incidentsSlice = createSlice({
  name: 'incidents',
  initialState,
  reducers: {
    setIncidents(state, action: PayloadAction<Incident[]>) {
      state.items = action.payload;
    },
    setLoading(state, action: PayloadAction<boolean>) {
      state.loading = action.payload;
    }
  },
});

export const { setIncidents, setLoading } = incidentsSlice.actions;

export const store = configureStore({
  reducer: {
    incidents: incidentsSlice.reducer,
  },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
