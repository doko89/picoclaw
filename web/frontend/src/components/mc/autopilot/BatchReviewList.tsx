'use client';

import { useState } from 'react';
import { IconCircleCheck, IconCircleX, IconHelp, IconBolt, IconCheck, IconLoader2 } from '@tabler/icons-react';
import { batchSwipe, type MCIdea } from '@/api/mc';

interface BatchReviewListProps {
  productId: string;
  ideas: MCIdea[];
  onComplete: () => void;
}

const ACTION_CONFIG = {
  approve: { icon: IconCircleCheck, label: 'Yes', color: 'text-green-400', bg: 'bg-green-500/10 border-green-500/30' },
  reject: { icon: IconCircleX, label: 'No', color: 'text-red-400', bg: 'bg-red-500/10 border-red-500/30' },
  maybe: { icon: IconHelp, label: 'Maybe', color: 'text-yellow-400', bg: 'bg-yellow-500/10 border-yellow-500/30' },
  fire: { icon: IconBolt, label: 'Build Now', color: 'text-orange-400', bg: 'bg-orange-500/10 border-orange-500/30' },
};

export function BatchReviewList({ productId, ideas, onComplete }: BatchReviewListProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set(ideas.map(i => i.id)));
  const [action, setAction] = useState<string>('approve');
  const [submitting, setSubmitting] = useState(false);

  function toggle(id: string) {
    setSelected(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (selected.size === ideas.length) setSelected(new Set());
    else setSelected(new Set(ideas.map(i => i.id)));
  }

  async function handleSubmit() {
    if (selected.size === 0) return;
    setSubmitting(true);
    try {
      const actions = Array.from(selected).map(idea_id => ({ idea_id, action }));
      await batchSwipe(productId, actions);
      onComplete();
    } catch { /* ignore */ }
    finally { setSubmitting(false); }
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium text-foreground">
          Review {ideas.length} Pending Ideas — Select All
        </h2>
        <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
          <input type="checkbox" checked={selected.size === ideas.length} onChange={toggleAll} className="rounded" />
          Select all
        </label>
      </div>

      {/* Action selector + bulk buttons */}
      <div className="flex items-center gap-2 flex-wrap">
        <div className="flex gap-1">
          {(['approve', 'reject', 'maybe', 'fire'] as const).map(act => {
            const cfg = ACTION_CONFIG[act];
            return (
              <button
                key={act}
                onClick={() => setAction(act)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${action === act ? cfg.bg + ' ' + cfg.color : 'border-border bg-background text-muted-foreground hover:text-foreground'}`}
              >
                <cfg.icon className="w-3.5 h-3.5" />
                {cfg.label}
              </button>
            );
          })}
        </div>

        <div className="ml-auto flex gap-2">
          <button
            onClick={handleSubmit}
            disabled={selected.size === 0 || submitting}
            className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
          >
            {submitting ? <IconLoader2 className="w-4 h-4 animate-spin" /> : <IconCheck className="w-4 h-4" />}
            Apply to {selected.size} idea{selected.size !== 1 ? 's' : ''}
          </button>
        </div>
      </div>

      {/* Idea rows */}
      <div className="space-y-1">
        {ideas.map(idea => (
          <div key={idea.id} className="flex items-center gap-3 px-4 py-3 rounded-lg border border-border hover:bg-muted/30 transition-colors">
            <input
              type="checkbox"
              checked={selected.has(idea.id)}
              onChange={() => toggle(idea.id)}
              className="rounded border-border text-primary focus:ring-primary"
            />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium text-foreground">{idea.title}</div>
              <div className="text-xs text-muted-foreground">{idea.description?.slice(0, 80)}{idea.description?.length > 80 ? '...' : ''}</div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <span className="text-xs px-2 py-0.5 rounded bg-muted text-muted-foreground">{idea.category}</span>
              <span className="text-xs font-mono text-muted-foreground">{Math.round((idea.priority || 0) * 100)}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}