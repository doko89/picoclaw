'use client';

import { useState, useEffect } from 'react';
import { IconCircleCheck, IconClock, IconCircleX, IconLoader2 } from '@tabler/icons-react';
import type { MCTask } from '@/api/mc';
import { getTasks } from '@/api/mc';

interface BuildQueueProps {
  productId: string;
}

const STATUS_CONFIG: Record<string, { icon: React.ReactNode; color: string; bg: string }> = {
  pending: { icon: <IconClock className="w-3.5 h-3.5" />, color: 'text-yellow-400', bg: 'bg-yellow-500/10' },
  dispatching: { icon: <IconLoader2 className="w-3.5 h-3.5 animate-spin" />, color: 'text-blue-400', bg: 'bg-blue-500/10' },
  running: { icon: <IconLoader2 className="w-3.5 h-3.5 animate-spin" />, color: 'text-blue-400', bg: 'bg-blue-500/10' },
  merged: { icon: <IconCircleCheck className="w-3.5 h-3.5" />, color: 'text-green-400', bg: 'bg-green-500/10' },
  built: { icon: <IconCircleCheck className="w-3.5 h-3.5" />, color: 'text-green-400', bg: 'bg-green-500/10' },
  shipped: { icon: <IconCircleCheck className="w-3.5 h-3.5" />, color: 'text-green-400', bg: 'bg-green-500/10' },
  failed: { icon: <IconCircleX className="w-3.5 h-3.5" />, color: 'text-red-400', bg: 'bg-red-500/10' },
  cancelled: { icon: <IconCircleX className="w-3.5 h-3.5" />, color: 'text-muted-foreground', bg: 'bg-muted' },
};

export function BuildQueue({ productId }: BuildQueueProps) {
  const [tasks, setTasks] = useState<MCTask[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getTasks({ status: 'pending' })
      .then(all => all.filter(t => t.product_id === productId))
      .then(setTasks)
      .catch(() => setTasks([]))
      .finally(() => setLoading(false));
  }, [productId]);

  if (loading) {
    return (
      <div className="space-y-2">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="h-14 rounded-lg bg-muted animate-pulse" />
        ))}
      </div>
    );
  }

  if (tasks.length === 0) {
    return (
      <div className="text-center py-8">
        <IconCircleCheck className="w-8 h-8 text-muted-foreground mx-auto mb-2" />
        <p className="text-sm text-muted-foreground">No tasks in build queue</p>
        <p className="text-xs text-muted-foreground mt-1">Approved ideas generate build tasks here</p>
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {tasks.map(task => {
        const cfg = STATUS_CONFIG[task.status] || STATUS_CONFIG.pending;
        return (
          <div key={task.id} className="flex items-center gap-3 px-4 py-3 rounded-lg border border-border hover:bg-muted/30 transition-colors cursor-pointer">
            <div className={`shrink-0 w-7 h-7 rounded-lg flex items-center justify-center ${cfg.bg}`}>
              <span className={cfg.color}>{cfg.icon}</span>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium text-foreground">{task.title}</div>
              <div className="text-xs text-muted-foreground mt-0.5">
                {task.assigned_agent_id ? `Assigned` : 'Unassigned'}
                {task.estimated_cost_usd ? ` · ~$${task.estimated_cost_usd.toFixed(2)}` : ''}
              </div>
            </div>
            <div className="shrink-0 text-xs text-muted-foreground">
              {task.status}
            </div>
          </div>
        );
      })}
    </div>
  );
}