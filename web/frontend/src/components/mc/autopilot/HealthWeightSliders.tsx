'use client';

import { useState, useEffect, useCallback } from 'react';
import { IconDeviceFloppy, IconLoader2, IconRefresh } from '@tabler/icons-react';
import { HealthBadge } from './HealthBadge';
import { getProductHealthScore, updateProductHealthWeights, type MCHealthWeightConfig } from '@/api/mc';

interface Props {
  productId: string;
  initialWeights?: MCHealthWeightConfig;
  onSaved?: (weights: MCHealthWeightConfig) => void;
}

const COMPONENTS: { key: string; label: string; color: string }[] = [
  { key: 'research', label: 'Research Freshness', color: '#58a6ff' },
  { key: 'pipeline', label: 'Pipeline Depth', color: '#a371f7' },
  { key: 'swipe', label: 'Swipe Velocity', color: '#d29922' },
  { key: 'build', label: 'Build Success', color: '#3fb950' },
  { key: 'cost', label: 'Cost Efficiency', color: '#db61a2' },
];

const DEFAULT_WEIGHTS: MCHealthWeightConfig = {
  research: 20,
  pipeline: 20,
  swipe: 20,
  build: 20,
  cost: 20,
  disabled: [],
};

export function HealthWeightSliders({ productId, initialWeights, onSaved }: Props) {
  const [weights, setWeights] = useState<MCHealthWeightConfig>(initialWeights || DEFAULT_WEIGHTS);
  const [previewScore, setPreviewScore] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getProductHealthScore(productId)
      .then(data => setPreviewScore(data.score.overall_score))
      .catch(() => {});
  }, [productId]);

  const handleToggle = useCallback((key: string) => {
    setWeights(prev => ({
      ...prev,
      disabled: prev.disabled.includes(key)
        ? prev.disabled.filter(k => k !== key)
        : [...prev.disabled, key],
    }));
  }, []);

  const handleReset = useCallback(() => {
    setWeights({ ...DEFAULT_WEIGHTS });
  }, []);

  async function handleSave() {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      await updateProductHealthWeights(productId, weights);
      onSaved?.(weights);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
      const data = await getProductHealthScore(productId);
      setPreviewScore(data.score.overall_score);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  }

  const totalWeight = COMPONENTS
    .filter(c => !weights.disabled.includes(c.key))
    .reduce((sum, c) => {
      const val = weights[c.key as keyof MCHealthWeightConfig];
      return sum + (typeof val === 'number' ? val : 0);
    }, 0);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">Health Score Weights</h3>
        <div className="flex items-center gap-2">
          {previewScore !== null && (
            <div className="flex items-center gap-2 mr-3">
              <span className="text-xs text-muted-foreground">Current:</span>
              <HealthBadge score={previewScore} size={32} />
            </div>
          )}
          <button onClick={handleReset} className="min-h-8 px-3 rounded-lg border border-border bg-background text-muted-foreground hover:text-foreground text-xs flex items-center gap-1">
            <IconRefresh className="w-3 h-3" />
            Reset
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className={`min-h-8 px-3 rounded-lg flex items-center gap-1.5 text-xs font-medium transition-colors ${
              saved ? 'bg-green-500/20 text-green-400' : 'bg-primary text-primary-foreground hover:bg-primary/90'
            }`}
          >
            {saving ? <IconLoader2 className="w-3 h-3 animate-spin" /> : <IconDeviceFloppy className="w-3 h-3" />}
            {saved ? 'Saved' : saving ? 'Saving...' : 'Save Weights'}
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-destructive/10 border border-destructive/30 rounded-lg px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="space-y-3">
        {COMPONENTS.map(comp => {
          const isDisabled = weights.disabled.includes(comp.key);
          const value = (weights[comp.key as keyof MCHealthWeightConfig] as number) || 0;

          return (
            <div key={comp.key} className={`flex items-center gap-3 p-3 rounded-lg border transition-all ${isDisabled ? 'border-border/30 bg-background/30 opacity-50' : 'border-border bg-background'}`}>
              <label className="flex items-center gap-2 cursor-pointer min-w-[160px]">
                <input
                  type="checkbox"
                  checked={!isDisabled}
                  onChange={() => handleToggle(comp.key)}
                  className="rounded border-border bg-background text-primary focus:ring-primary"
                />
                <span className="text-sm font-medium" style={{ color: isDisabled ? '#8b949e' : comp.color }}>
                  {comp.label}
                </span>
              </label>
              <input
                type="range"
                min={0}
                max={100}
                value={value}
                onChange={e => setWeights(prev => ({ ...prev, [comp.key]: Number(e.target.value) }))}
                disabled={isDisabled}
                className="flex-1 h-1.5 bg-muted rounded-lg appearance-none cursor-pointer disabled:opacity-30"
                style={{ accentColor: isDisabled ? undefined : comp.color }}
              />
              <span className="text-sm font-mono text-muted-foreground w-10 text-right">
                {isDisabled ? '—' : `${value}%`}
              </span>
            </div>
          );
        })}
      </div>

      <div className="text-xs text-muted-foreground flex items-center justify-between pt-1">
        <span>
          Total active weight: <span className="font-mono">{totalWeight}%</span>
          {totalWeight !== 100 && totalWeight > 0 && (
            <span className="text-yellow-400 ml-1">(will be normalized to 100%)</span>
          )}
        </span>
        <span>{weights.disabled.length} component{weights.disabled.length !== 1 ? 's' : ''} disabled</span>
      </div>
    </div>
  );
}