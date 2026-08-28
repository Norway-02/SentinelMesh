import React from 'react';
import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Terminal,
  PlayCircle,
  GitBranch,
  Server,
  BrainCircuit,
  Bot,
  Network,
  FlaskConical,
  Activity,
  Settings as SettingsIcon,
} from 'lucide-react';

export const Sidebar: React.FC = () => {
  const navGroups = [
    {
      title: 'OPERATIONS',
      items: [
        { path: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
        { path: '/tasks', label: 'Tasks', icon: Terminal },
        { path: '/runs', label: 'Runs', icon: PlayCircle },
      ],
    },
    {
      title: 'INTELLIGENCE',
      items: [
        { path: '/router', label: 'Model Router', icon: GitBranch },
        { path: '/providers', label: 'Providers', icon: Server },
        { path: '/intelligence', label: 'Adaptive Intelligence', icon: BrainCircuit },
      ],
    },
    {
      title: 'CONTROL PLANE',
      items: [
        { path: '/agents', label: 'Agents', icon: Bot },
        { path: '/distributed', label: 'Distributed', icon: Network },
      ],
    },
    {
      title: 'ANALYSIS',
      items: [
        { path: '/experiments', label: 'Experiments', icon: FlaskConical },
        { path: '/events', label: 'Events', icon: Activity },
      ],
    },
    {
      title: 'SYSTEM',
      items: [
        { path: '/settings', label: 'Settings', icon: SettingsIcon },
      ],
    },
  ];

  return (
    <aside
      style={{
        width: '230px',
        backgroundColor: 'var(--bg-secondary)',
        borderRight: '1px solid var(--border-color)',
        display: 'flex',
        flexDirection: 'column',
        height: '100vh',
        position: 'fixed',
        left: 0,
        top: 0,
        zIndex: 40,
      }}
    >
      {/* Geometric SentinelMesh Brand Header */}
      <div
        style={{
          padding: '20px 18px',
          borderBottom: '1px solid var(--border-color)',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
        }}
      >
        <div
          style={{
            width: '32px',
            height: '32px',
            borderRadius: 'var(--radius-sm)',
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-color-active)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'var(--accent-blue)',
            fontWeight: 800,
            fontFamily: 'var(--font-mono)',
            fontSize: '15px',
            boxShadow: 'var(--shadow-glow-blue)',
          }}
        >
          [S]
        </div>
        <div>
          <div style={{ fontWeight: 700, fontSize: '14px', letterSpacing: '-0.02em', color: 'var(--text-primary)', fontFamily: 'var(--font-sans)' }}>
            SENTINELMESH
          </div>
          <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', letterSpacing: '0.05em', textTransform: 'uppercase' }}>
            CONTROL PLANE
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav style={{ flex: 1, padding: '18px 12px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '18px' }}>
        {navGroups.map((group) => (
          <div key={group.title}>
            <div
              style={{
                fontSize: '10px',
                fontWeight: 700,
                color: 'var(--text-muted)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                padding: '0 8px 6px 8px',
              }}
            >
              {group.title}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <NavLink
                    key={item.path}
                    to={item.path}
                    style={({ isActive }) => ({
                      display: 'flex',
                      alignItems: 'center',
                      gap: '10px',
                      padding: '8px 12px',
                      borderRadius: 'var(--radius-sm)',
                      fontSize: '13px',
                      fontWeight: isActive ? 600 : 500,
                      color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
                      backgroundColor: isActive ? 'rgba(110, 168, 254, 0.10)' : 'transparent',
                      borderLeft: isActive ? '2px solid var(--accent-blue)' : '2px solid transparent',
                      transition: 'all 0.15s ease',
                      textDecoration: 'none',
                    })}
                  >
                    <Icon size={15} style={{ color: 'inherit' }} />
                    <span>{item.label}</span>
                  </NavLink>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* Footer System Meta */}
      <div
        style={{
          padding: '14px 16px',
          borderTop: '1px solid var(--border-color)',
          fontSize: '10px',
          color: 'var(--text-dim)',
          fontFamily: 'var(--font-mono)',
          display: 'flex',
          flexDirection: 'column',
          gap: '2px',
        }}
      >
        <div>API: 127.0.0.1:8787</div>
        <div>ENGINE: STAGE 17 - MODEL ROUTER</div>
      </div>
    </aside>
  );
};
