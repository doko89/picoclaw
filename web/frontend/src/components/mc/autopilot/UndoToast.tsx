'use client';

import { useState, useEffect } from 'react';
import { IconArrowBackUp, IconLoader2 } from '@tabler/icons-react';
import type { MCIdea } from '@/api/mc-products';

interface UndoToastProps {
  swipeId: string;
  productId: string;
  idea: MCIdea;
  onUndo: (idea: MCIdea) => void;
  onDismiss: () => void;
}

export function UndoToast({ swipeId, productId, idea, onUndo, onDismiss }: UndoToastProps) {
  const [secondsLeft, setSecondsLeft] = useState(10);
  const [undoing, setUndoing] = useState(false);

  useEffect(() => {
    const interval = setInterval(() => {
      setSecondsLeft(prev => {
        if (prev <= 1) {
          clearInterval(interval);
          onDismiss();
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [onDismiss]);

  async function handleUndo() {
    setUndoing(true);
    try {
      const res = await fetch(`/api/mc/products/${productId}/swipe/${swipeId}/undo`, { method: 'DELETE' });
      if (res.ok) {
        const data = await res.json();
        onUndo(data.idea);
        onDismiss();
      }
    } catch {
      setUndoing(false);
    }
  }

  const radius = 12;
  const circumference = 2 * Math.PI * radius;
  const progress = (secondsLeft / 10) * circumference;
  const offset = circumference - progress;

  return (
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-3 bg-secondary border border-border rounded-xl px-4 py-3 shadow-xl">
      {/* Countdown ring */}
      <div className="relative w-6 h-6">
        <svg width={24} height={24} viewBox="0 0 24 24" className="transform -rotate-90">
          <circle cx={12} cy={12} r={radius} fill="none" stroke="rgba(255,255,255,0.1)" strokeWidth={2} />
          <circle
            cx={12} cy={12} r={radius}
            fill="none"
            stroke="#58a6ff"
            strokeWidth={2}
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            strokeLinecap="round"
            style={{ transition: 'stroke-dashoffset 1s linear' }}
          />
        </svg>
      </div>

      <div className="text-sm">
        <span className="text-foreground font-medium">Swiped {idea.status === 'approved' ? 'yes' : idea.status === 'rejected' ? 'no' : idea.status}</span>
        <span className="text-muted-foreground ml-1">— undo in {secondsLeft}s</span>
      </div>

      <button
        onClick={handleUndo}
        disabled={undoing}
        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary/10 text-primary hover:bg-primary/20 text-sm font-medium transition-colors disabled:opacity-50"
      >
        {undoing ? <IconLoader2 className="w-3.5 h-3.5 animate-spin" /> : <IconArrowBackUp className="w-3.5 h-3.5" />}
        Undo
      </button>
    </div>
  );
}