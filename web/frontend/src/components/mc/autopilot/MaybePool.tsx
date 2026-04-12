'use client';

import { useState, useEffect } from 'react';
import { IconRefresh, IconHelp, IconLoader2 } from '@tabler/icons-react';
import { getMaybePool, resurfaceIdea, type MCMaybePoolEntry } from '@/api/mc';

interface MaybePoolProps {
  productId: string;
}

function relativeDate(iso: string | null): string {
  if (!iso) return 'Not scheduled';
  const diff = Date.now() - new Date(iso).getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  if (days < 0) return `In ${Math.abs(days)} days`;
  return `${days} days ago`;
}

export function MaybePool({ productId }: MaybePoolProps) {
  const [entries, setEntries] = useState<MCMaybePoolEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [resurfacing, setResurfacing] = useState<string | null>(null);

  function load() {
    setLoading(true);
    getMaybePool(productId)
      .then(setEntries)
      .catch(() => setEntries([]))
      .finally(() => setLoading(false));
  }

  useEffect(() => { load(); }, [productId]);

  async function handleResurface(entry: MCMaybePoolEntry) {
    setResurfacing(entry.id);
    try {
      await resurfaceIdea(productId, entry.id, 'Manual resurface from maybe pool');
      setEntries(prev => prev.filter(e => e.id !== entry.id));
    } catch { /* ignore */ }
    finally { setResurfacing(null); }
  }

  if (loading) {
    return (
      <div className="space-y-2">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="h-16 rounded-lg bg-muted animate-pulse" />
        ))}
      </div>
    );
  }

  if (entries.length === 0) {
    return (
      <div className="text-center py-8">
        <IconHelp className="w-8 h-8 text-muted-foreground mx-auto mb-2" />
        <p className="text-sm text-muted-foreground">No ideas in maybe pool</p>
        <p className="text-xs text-muted-foreground mt-1">Ideas swiped "maybe" appear here for re-evaluation</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-muted-foreground">{entries.length} idea{entries.length !== 1 ? 's' : ''} pending re-evaluation</span>
      </div>
      {entries.map(entry => (
        <div key={entry.id} className="flex items-start gap-3 p-3 rounded-lg border border-border bg-background">
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium text-foreground">{entry.idea_title}</div>
            <div className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{entry.idea_description}</div>
            <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
              <span className={`px-1.5 py-0.5 rounded bg-muted`}>{entry.idea_category}</span>
              <span>Eval #{entry.evaluation_count}</span>
              <span>Next: {relativeDate(entry.next_evaluate_at)}</span>
            </div>
          </div>
          <button
            onClick={() => handleResurface(entry)}
            disabled={resurfacing === entry.id}
            className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary/10 text-primary hover:bg-primary/20 text-xs font-medium transition-colors disabled:opacity-50"
          >
            {resurfacing === entry.id ? <IconLoader2 className="w-3 h-3 animate-spin" /> : <IconRefresh className="w-3 h-3" />}
            Resurface
          </button>
        </div>
      ))}
    </div>
  );
}