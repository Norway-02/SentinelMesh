import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { SSEEvent } from '../../api/types';
import { formatTimestamp } from '../../utils/formatters';
import { Radio, RefreshCw, Code, Eye, Search } from 'lucide-react';

export const Events: React.FC = () => {
  const [stageFilter, setStageFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedEvent, setSelectedEvent] = useState<SSEEvent | null>(null);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['eventsList', stageFilter, typeFilter, searchQuery],
    queryFn: () =>
      api.getEvents({
        limit: 100,
        stage: stageFilter || undefined,
        type: typeFilter || undefined,
        task_id: searchQuery || undefined,
      }),
    refetchInterval: 3000,
  });

  const events = data?.events || [];

  const getEventType = (evt: SSEEvent) => evt.event_type || evt.type || 'EVENT';
  const getEventTaskId = (evt: SSEEvent) => evt.aggregate_id || evt.task_id || evt.id || 'N/A';
  const getEventTime = (evt: SSEEvent) => formatTimestamp(evt.occurred_at || evt.timestamp);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '22px', fontWeight: 700, letterSpacing: '-0.02em' }}>
            Event Stream &amp; Audit Inspector
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
            Real-time domain event log across Stage 17, 18, and 19 pipeline executions
          </p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)', fontWeight: 600, backgroundColor: 'var(--badge-green-bg)', padding: '4px 10px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--badge-green-border)' }}>
            <Radio size={12} className="animate-pulse" /> SSE LIVE
          </div>
          <button
            onClick={() => refetch()}
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-color)',
              color: 'var(--text-primary)',
              padding: '8px 14px',
              borderRadius: 'var(--radius-md)',
              cursor: 'pointer',
              fontSize: '13px',
              fontWeight: 500,
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            <RefreshCw size={14} /> Refresh
          </button>
        </div>
      </div>

      {/* Filters Bar */}
      <div
        style={{
          backgroundColor: 'var(--bg-card)',
          border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)',
          padding: '14px 20px',
          display: 'grid',
          gridTemplateColumns: '1fr 1fr 2fr 100px',
          gap: '12px',
          alignItems: 'center',
        }}
      >
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>PIPELINE STAGE</label>
          <select
            value={stageFilter}
            onChange={(e) => setStageFilter(e.target.value)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          >
            <option value="">All Stages</option>
            <option value="stage17">Stage 17 (Router)</option>
            <option value="stage18">Stage 18 (Adaptive)</option>
            <option value="stage19">Stage 19 (Policy)</option>
            <option value="agent">Agent Lifecycle</option>
            <option value="run">Run Execution</option>
          </select>
        </div>

        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>EVENT TYPE</label>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          >
            <option value="">All Event Types</option>
            <option value="sentinel.router.v1.decided">ROUTING_DECIDED</option>
            <option value="sentinel.router.v1.invocation_completed">INVOCATION_COMPLETED</option>
            <option value="sentinel.adaptive.v1.decided">ADAPTIVE_DECIDED</option>
            <option value="sentinel.adaptive.v1.drift_detected">DRIFT_DETECTED</option>
            <option value="sentinel.policy.v2.decided">POLICY_DECIDED</option>
            <option value="sentinel.policy.v2.rollback_triggered">POLICY_ROLLBACK</option>
          </select>
        </div>

        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>SEARCH</label>
          <div style={{ position: 'relative' }}>
            <input
              type="text"
              placeholder="Search Task ID, Model ID, or Payload..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px 10px 6px 28px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}
            />
            <Search size={12} style={{ position: 'absolute', left: '10px', top: '9px', color: 'var(--text-muted)' }} />
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'flex-end', height: '100%' }}>
          <button
            onClick={() => { setStageFilter(''); setTypeFilter(''); setSearchQuery(''); }}
            style={{ width: '100%', backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border-color)', color: 'var(--text-muted)', borderRadius: 'var(--radius-sm)', padding: '6px', fontSize: '11px', cursor: 'pointer' }}
          >
            Reset
          </button>
        </div>
      </div>

      {/* Events Table & Payload Inspector Side-by-Side */}
      <div style={{ display: 'grid', gridTemplateColumns: selectedEvent ? '1fr 1fr' : '1fr', gap: '20px' }}>
        {/* Events List */}
        <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '12px', letterSpacing: '0.05em' }}>
            RECORDED EVENTS ({events.length})
          </div>

          {isLoading ? (
            <div style={{ padding: '30px 0', textAlign: 'center', color: 'var(--text-muted)' }}>Loading events...</div>
          ) : events.length === 0 ? (
            <div style={{ padding: '30px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
              No events match the current filter.
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '600px', overflowY: 'auto' }}>
              {events.map((evt, idx) => (
                <div
                  key={evt.id || idx}
                  onClick={() => setSelectedEvent(evt)}
                  style={{
                    padding: '10px 14px',
                    borderRadius: 'var(--radius-sm)',
                    backgroundColor: selectedEvent?.id === evt.id ? 'var(--bg-surface-hover)' : 'var(--bg-surface)',
                    border: `1px solid ${selectedEvent?.id === evt.id ? 'var(--accent-blue)' : 'var(--border-color)'}`,
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                  }}
                >
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{ fontSize: '12px', fontWeight: 700, color: 'var(--accent-cyan)', fontFamily: 'var(--font-mono)' }}>
                        {getEventType(evt)}
                      </span>
                      {evt.stage && (
                        <span style={{ fontSize: '10px', fontWeight: 700, padding: '1px 5px', borderRadius: 'var(--radius-sm)', backgroundColor: 'var(--bg-input)', color: 'var(--text-muted)', border: '1px solid var(--border-color)' }}>
                          {evt.stage.toUpperCase()}
                        </span>
                      )}
                    </div>
                    <div style={{ fontSize: '11px', color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
                      ID: {getEventTaskId(evt)}
                    </div>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <span style={{ fontSize: '11px', color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
                      {getEventTime(evt)}
                    </span>
                    <Eye size={14} style={{ color: 'var(--text-muted)' }} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Selected Event Payload Inspector */}
        {selectedEvent && (
          <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px', display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Code size={18} style={{ color: 'var(--accent-purple)' }} />
                <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Event Payload Inspector</h3>
              </div>
              <button
                onClick={() => setSelectedEvent(null)}
                style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '12px' }}
              >
                Close
              </button>
            </div>

            <div style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div><strong style={{ color: 'var(--text-muted)' }}>Event ID:</strong> <span style={{ color: 'var(--text-primary)' }}>{selectedEvent.id}</span></div>
              <div><strong style={{ color: 'var(--text-muted)' }}>Event Type:</strong> <span style={{ color: 'var(--accent-cyan)' }}>{getEventType(selectedEvent)}</span></div>
              <div><strong style={{ color: 'var(--text-muted)' }}>Timestamp:</strong> <span style={{ color: 'var(--accent-emerald)' }}>{getEventTime(selectedEvent)}</span></div>
              <div><strong style={{ color: 'var(--text-muted)' }}>Task ID:</strong> <span style={{ color: 'var(--text-primary)' }}>{getEventTaskId(selectedEvent)}</span></div>
            </div>

            <div style={{ flex: 1 }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '6px' }}>RAW EVENT PAYLOAD</div>
              <pre
                style={{
                  padding: '14px',
                  backgroundColor: 'var(--bg-input)',
                  border: '1px solid var(--border-color)',
                  borderRadius: 'var(--radius-sm)',
                  fontSize: '11px',
                  fontFamily: 'var(--font-mono)',
                  color: 'var(--accent-cyan)',
                  maxHeight: '450px',
                  overflowY: 'auto',
                }}
              >
                {JSON.stringify(selectedEvent.payload || selectedEvent, null, 2)}
              </pre>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
