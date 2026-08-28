import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from './components/Layout/Layout';
import { Dashboard } from './pages/Dashboard/Dashboard';
import { Tasks } from './pages/Tasks/Tasks';
import { Router } from './pages/Router/Router';
import { Providers } from './pages/Providers/Providers';
import { Intelligence } from './pages/Intelligence/Intelligence';
import { Agents } from './pages/Agents/Agents';
import { Distributed } from './pages/Distributed/Distributed';
import { Experiments } from './pages/Experiments/Experiments';
import { Events } from './pages/Events/Events';
import { Settings } from './pages/Settings/Settings';
import { RunsList, RunDetails } from './pages/Runs/RunDetails';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 5000,
    },
  },
});

export const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Layout>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/tasks" element={<Tasks />} />
            <Route path="/runs" element={<RunsList />} />
            <Route path="/runs/:id" element={<RunDetails />} />
            <Route path="/router" element={<Router />} />
            <Route path="/providers" element={<Providers />} />
            <Route path="/intelligence" element={<Intelligence />} />
            <Route path="/agents" element={<Agents />} />
            <Route path="/distributed" element={<Distributed />} />
            <Route path="/experiments" element={<Experiments />} />
            <Route path="/events" element={<Events />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </QueryClientProvider>
  );
};

export default App;
