'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { IconActivity, IconChevronRight, IconChevronLeft, IconBolt, IconClock, IconCircleCheck, IconAlertCircle, IconLoader2, IconX } from '@tabler/icons-react';
import { getActivityLog, type MCActivityEntry } from '@/api/mc';

interface ActivityPanelProps {
  productId: string;
}

const EVENT_ICONS: Record<string, React.ReactNode> = {
  phase_init: <IconBolt className="w-3.5 h-3.5 text-blue-400" />,
  phase_llm_submitted: <IconLoader2 className="w-3.5 h-3.5 text-yellow-400" />,
  phase_llm_polling: <IconClock className="w-3.5 h-3.5 text-yellow-400 animate-spin" />,
  phase_report_received: <IconCircleCheck className="w-3.5 h-3.5 text-green-400" />,
  phase_ideas_parsed: <IconBolt className="w-3.5 h-3.5 text-purple-400" />,
  phase_ideas_stored: <IconCircleCheck className="w-3.5 h-3.5 text-green-400" />,
  phase_completed: <IconCircleCheck className="w-3.5 h-3.5 text-green-400" />,
  idea_stored: <IconBolt className="w-3.5 h-3.5 text-purple-400" />,
  error: <IconAlertCircle className="w-3.5 h-3.5 text-red-400" />,
  recovery_completed: <IconCircleCheck className="w-3.5 h-3.5 text-green-400" />,
  ideas_generated: <IconBolt className="w-3.5 h-3.5 text-purple-400" />,
};

function getEventIcon(eventType: string) {
  return EVENT_ICONS[eventType] || <IconActivity className="w-3.5 h-3.5 text-muted-foreground" />;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function groupByCycle(entries: MCActivityEntry[]): Map<string, MCActivityEntry[]> {
  const groups = new Map<string, MCActivityEntry[]>();
  for (const entry of entries) {
    const key = `${entry.cycle_type}-${entry.cycle_id}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(entry);
  }
  return groups;
}

function cycleLabel(cycleType: string, index: number): string {
  const type = cycleType === 'research' ? 'Research' : 'Ideation';
  return `${type} Cycle #${index}`;
}

export function ActivityPanel({ productId }: ActivityPanelProps) {
  const [entries, setEntries] = useState<MCActivityEntry[]>([]);
  const [isOpen, setIsOpen] = useState(() => {
    if (typeof window === 'undefined') return true;
    return localStorage.getItem(`autopilot-activity-open-${productId}`) !== 'false';
  });
  const bottomRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const initialLoadDone = useRef(false);

  useEffect(() => {
    localStorage.setItem(`autopilot-activity-open-${productId}`, String(isOpen));
  }, [isOpen, productId]);

  useEffect(() => {
    getActivityLog(productId, 50)
      .then(data => {
        setEntries((data.entries || []).reverse());
        requestAnimationFrame(() => { initialLoadDone.current = true; });
      })
      .catch(() => {});
  }, [productId]);

  const handleSSEMessage = useCallback((event: MessageEvent) => {
    try {
      if (event.data.startsWith(':')) return;
      const sseEvent = JSON.parse(event.data);
      if (sseEvent.type === 'autopilot_activity' && sseEvent.payload?.product_id === productId) {
        setEntries(prev => [...prev, sseEvent.payload as MCActivityEntry]);
      }
    } catch { /* ignore */ }
  }, [productId]);

  useEffect(() => {
    const es = new EventSource('/api/mc/events/stream');
    eventSourceRef.current = es;
    es.onmessage = handleSSEMessage;
    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [handleSSEMessage]);

  useEffect(() => {
    if (initialLoadDone.current) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [entries]);

  const grouped = groupByCycle(entries);
  const cycleKeys = Array.from(grouped.keys());

  const [mobileOpen, setMobileOpen] = useState(false);

  const panelContent = (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-3 py-2 border-b border-border">
        <div className="flex items-center gap-2">
          <IconActivity className="w-4 h-4 text-primary" />
          <span className="text-sm font-medium">Activity</span>
          <span className="text-xs text-muted-foreground">({entries.length})</span>
        </div>
        <button onClick={() => setIsOpen(false)} className="hidden lg:block text-muted-foreground hover:text-foreground">
          <IconChevronRight className="w-4 h-4" />
        </button>
        <button onClick={() => setMobileOpen(false)} className="lg:hidden text-muted-foreground hover:text-foreground">
          <IconX className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-3">
        {entries.length === 0 && (
          <p className="text-xs text-muted-foreground text-center py-4">No activity yet</p>
        )}

        {cycleKeys.map((key, idx) => {
          const group = grouped.get(key)!;
          const cycleType = key.split('-')[0];

          return (
            <div key={key}>
              <div className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider mb-1">
                {cycleLabel(cycleType, cycleKeys.length - idx)}
              </div>
              <div className="space-y-1">
                {group.map(entry => (
                  <div key={entry.id} className="flex items-start gap-2 text-xs group">
                    <div className="mt-0.5 shrink-0">{getEventIcon(entry.event_type)}</div>
                    <div className="flex-1 min-w-0">
                      <span className="text-foreground">{entry.message}</span>
                      {entry.detail && (
                        <span className="text-muted-foreground ml-1">— {entry.detail}</span>
                      )}
                      {entry.cost_usd != null && entry.cost_usd > 0 && (
                        <span className="text-green-400 ml-1">${entry.cost_usd.toFixed(4)}</span>
                      )}
                    </div>
                    <span className="text-[10px] text-muted-foreground shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                      {relativeTime(entry.created_at)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
        <div ref={bottomRef} />
      </div>
    </div>
  );

  return (
    <>
      {isOpen ? (
        <div className="hidden lg:flex w-80 border-l border-border bg-secondary flex-col">
          {panelContent}
        </div>
      ) : (
        <button
          onClick={() => setIsOpen(true)}
          className="hidden lg:flex items-center justify-center w-8 border-l border-border bg-secondary hover:bg-muted text-muted-foreground hover:text-foreground"
          title="Show activity panel"
        >
          <IconChevronLeft className="w-4 h-4" />
        </button>
      )}

      <button
        onClick={() => setMobileOpen(true)}
        className="lg:hidden fixed bottom-4 right-4 z-40 w-12 h-12 rounded-full bg-primary text-primary-foreground shadow-lg flex items-center justify-center"
      >
        <IconActivity className="w-5 h-5" />
        {entries.length > 0 && (
          <span className="absolute -top-1 -right-1 w-5 h-5 rounded-full bg-red-500 text-white text-[10px] flex items-center justify-center">
            {Math.min(entries.length, 99)}
          </span>
        )}
      </button>

      {mobileOpen && (
        <div className="lg:hidden fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/50" onClick={() => setMobileOpen(false)} />
          <div className="absolute right-0 top-0 bottom-0 w-80 bg-secondary">
            {panelContent}
          </div>
        </div>
      )}
    </>
  );
}