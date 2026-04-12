'use client';

import { useRef, useEffect } from 'react';
import type { MCProductHealthScore } from '@/api/mc';

interface HealthChartProps {
  history: MCProductHealthScore[];
  component: 'overall' | 'research_freshness' | 'pipeline_depth' | 'swipe_velocity' | 'build_success' | 'cost_efficiency';
  label: string;
  color: string;
}

const SCORE_KEY_MAP: Record<string, keyof MCProductHealthScore> = {
  overall: 'overall_score',
  research_freshness: 'research_freshness_score',
  pipeline_depth: 'pipeline_depth_score',
  swipe_velocity: 'swipe_velocity_score',
  build_success: 'build_success_score',
  cost_efficiency: 'cost_efficiency_score',
};

// Simple canvas-based line chart (no ECharts dependency in PicoClaw frontend)
export function HealthChart({ history, component, label, color }: HealthChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || history.length === 0) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const key = SCORE_KEY_MAP[component];
    const data = history.map(h => ({ date: (h.snapshot_date || h.calculated_at?.split('T')[0] || ''), value: (h[key] as number) ?? 0 }));

    const width = canvas.offsetWidth;
    const height = 180;
    canvas.width = width * devicePixelRatio;
    canvas.height = height * devicePixelRatio;
    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;
    ctx.scale(devicePixelRatio, devicePixelRatio);

    ctx.clearRect(0, 0, width, height);

    const padding = { top: 20, right: 16, bottom: 30, left: 36 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;

    // Draw grid lines
    ctx.strokeStyle = '#21262d';
    ctx.lineWidth = 0.5;
    ctx.setLineDash([2, 2]);
    for (let i = 0; i <= 4; i++) {
      const y = padding.top + (chartH / 4) * i;
      ctx.beginPath();
      ctx.moveTo(padding.left, y);
      ctx.lineTo(width - padding.right, y);
      ctx.stroke();
    }
    ctx.setLineDash([]);

    // Draw axes labels
    ctx.fillStyle = '#8b949e';
    ctx.font = '10px system-ui';
    ctx.textAlign = 'center';
    for (let i = 0; i <= 4; i++) {
      const y = padding.top + (chartH / 4) * i;
      ctx.fillText(Math.round(100 - i * 25).toString(), padding.left - 8, y + 4);
    }

    // Draw data points
    const xStep = chartW / Math.max(data.length - 1, 1);
    const yScale = (v: number) => padding.top + chartH - (v / 100) * chartH;

    // Area fill
    ctx.beginPath();
    ctx.moveTo(padding.left, height - padding.bottom);
    data.forEach((d, i) => {
      const x = padding.left + i * xStep;
      const y = yScale(d.value);
      if (i === 0) ctx.lineTo(x, y);
      else {
        const prevX = padding.left + (i - 1) * xStep;
        const prevY = yScale(data[i - 1].value);
        const cpX = (prevX + x) / 2;
        ctx.bezierCurveTo(cpX, prevY, cpX, y, x, y);
      }
    });
    ctx.lineTo(padding.left + (data.length - 1) * xStep, height - padding.bottom);
    ctx.closePath();
    const grad = ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom);
    grad.addColorStop(0, color + '33');
    grad.addColorStop(1, color + '05');
    ctx.fillStyle = grad;
    ctx.fill();

    // Line
    ctx.beginPath();
    data.forEach((d, i) => {
      const x = padding.left + i * xStep;
      const y = yScale(d.value);
      if (i === 0) ctx.moveTo(x, y);
      else {
        const prevX = padding.left + (i - 1) * xStep;
        const prevY = yScale(data[i - 1].value);
        const cpX = (prevX + x) / 2;
        ctx.bezierCurveTo(cpX, prevY, cpX, y, x, y);
      }
    });
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.stroke();

    // Dots
    data.forEach((d, i) => {
      const x = padding.left + i * xStep;
      const y = yScale(d.value);
      ctx.beginPath();
      ctx.arc(x, y, data.length <= 15 ? 4 : 2, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();
    });

    // X axis labels (first, middle, last)
    ctx.fillStyle = '#8b949e';
    ctx.font = '9px system-ui';
    ctx.textAlign = 'center';
    if (data.length > 0) {
      ctx.fillText(data[0].date, padding.left, height - 8);
      if (data.length > 1) {
        ctx.fillText(data[Math.floor(data.length / 2)].date, padding.left + (data.length - 1) / 2 * xStep, height - 8);
        ctx.fillText(data[data.length - 1].date, padding.left + (data.length - 1) * xStep, height - 8);
      }
    }

    // Tooltip on hover
    const handleMouseMove = (e: MouseEvent) => {
      const rect = canvas.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      for (let i = 0; i < data.length; i++) {
        const x = padding.left + i * xStep;
        if (Math.abs(mx - x) < 10 && Math.abs(my - yScale(data[i].value)) < 10) {
          canvas.title = `${data[i].date}: ${data[i].value}`;
          return;
        }
      }
      canvas.title = '';
    };
    canvas.addEventListener('mousemove', handleMouseMove);

    return () => canvas.removeEventListener('mousemove', handleMouseMove);
  }, [history, component, color, label]);

  return (
    <div className="w-full">
      <div className="text-xs text-muted-foreground mb-1">{label}</div>
      <canvas ref={canvasRef} className="w-full" style={{ height: 180 }} />
    </div>
  );
}