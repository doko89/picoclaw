'use client';

import { useState, useEffect } from 'react';
import { IconCircleCheck, IconCircleX, IconHelp, IconBolt, IconChevronDown, IconChevronUp } from '@tabler/icons-react';
import type { MCIdea } from '@/api/mc-products';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

interface IdeasListProps {
  productId: string;
  initialStatus?: string;
  initialCategory?: string;
}

const CATEGORY_COLORS: Record<string, string> = {
  feature: 'bg-blue-500/10 text-blue-400',
  improvement: 'bg-green-500/10 text-green-400',
  ux: 'bg-purple-500/10 text-purple-400',
  performance: 'bg-yellow-500/10 text-yellow-400',
  integration: 'bg-pink-500/10 text-pink-400',
  infrastructure: 'bg-gray-500/10 text-gray-400',
  content: 'bg-cyan-500/10 text-cyan-400',
  growth: 'bg-orange-500/10 text-orange-400',
  monetization: 'bg-red-500/10 text-red-400',
  operations: 'bg-indigo-500/10 text-indigo-400',
  security: 'bg-red-600/10 text-red-400',
};

const STATUS_ICONS: Record<string, React.ReactNode> = {
  pending: <IconHelp className="w-3.5 h-3.5 text-muted-foreground" />,
  approved: <IconCircleCheck className="w-3.5 h-3.5 text-green-400" />,
  rejected: <IconCircleX className="w-3.5 h-3.5 text-red-400" />,
  maybe: <IconHelp className="w-3.5 h-3.5 text-yellow-400" />,
  building: <IconBolt className="w-3.5 h-3.5 text-orange-400 animate-pulse" />,
};

export function IdeasList({ productId, initialStatus, initialCategory }: IdeasListProps) {
  const [ideas, setIdeas] = useState<MCIdea[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState(initialStatus || '');
  const [categoryFilter, setCategoryFilter] = useState(initialCategory || '');
  const [sortBy, setSortBy] = useState<'created_at' | 'priority' | 'category'>('created_at');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(0);
  const pageSize = 20;

  useEffect(() => {
    setLoading(true);
    const params = new URLSearchParams();
    if (statusFilter) params.set('status', statusFilter);
    if (categoryFilter) params.set('category', categoryFilter);
    params.set('limit', '100');

    fetch(`/api/mc/products/${productId}/ideas?${params}`)
      .then(r => r.ok ? r.json() : [])
      .then(data => {
        setIdeas(data);
        setPage(0);
      })
      .finally(() => setLoading(false));
  }, [productId, statusFilter, categoryFilter]);

  const sorted = [...ideas].sort((a, b) => {
    let cmp = 0;
    if (sortBy === 'priority') cmp = b.priority - a.priority;
    else if (sortBy === 'category') cmp = a.category.localeCompare(b.category);
    else cmp = new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    return sortDir === 'desc' ? cmp : -cmp;
  });

  const paginated = sorted.slice(0, (page + 1) * pageSize);
  const hasMore = sorted.length > paginated.length;

  function toggleSort(col: typeof sortBy) {
    if (sortBy === col) setSortDir(d => d === 'desc' ? 'asc' : 'desc');
    else { setSortBy(col); setSortDir('desc'); }
  }

  return (
    <div className="space-y-3">
      {/* Filters */}
      <div className="flex items-center gap-2 flex-wrap">
        <Select value={statusFilter || undefined} onValueChange={val => setStatusFilter(val === "__all__" ? "" : val)}>
          <SelectTrigger className="w-[140px] h-7 text-xs">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All statuses</SelectItem>
            <SelectItem value="pending">Pending</SelectItem>
            <SelectItem value="approved">Approved</SelectItem>
            <SelectItem value="rejected">Rejected</SelectItem>
            <SelectItem value="maybe">Maybe</SelectItem>
            <SelectItem value="building">Building</SelectItem>
          </SelectContent>
        </Select>
        <Select value={categoryFilter || undefined} onValueChange={val => setCategoryFilter(val === "__all__" ? "" : val)}>
          <SelectTrigger className="w-[150px] h-7 text-xs">
            <SelectValue placeholder="All categories" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All categories</SelectItem>
            <SelectItem value="feature">Feature</SelectItem>
            <SelectItem value="improvement">Improvement</SelectItem>
            <SelectItem value="ux">UX</SelectItem>
            <SelectItem value="performance">Performance</SelectItem>
            <SelectItem value="integration">Integration</SelectItem>
            <SelectItem value="infrastructure">Infrastructure</SelectItem>
            <SelectItem value="content">Content</SelectItem>
            <SelectItem value="growth">Growth</SelectItem>
            <SelectItem value="monetization">Monetization</SelectItem>
            <SelectItem value="operations">Operations</SelectItem>
            <SelectItem value="security">Security</SelectItem>
          </SelectContent>
        </Select>
        <span className="text-xs text-muted-foreground ml-auto">{ideas.length} ideas</span>
      </div>

      {/* Table header */}
      <div className="grid grid-cols-12 gap-2 px-3 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">
        <div className="col-span-5 flex items-center gap-1 cursor-pointer hover:text-foreground" onClick={() => toggleSort('category')}>
          Idea {sortBy === 'category' ? (sortDir === 'desc' ? <IconChevronDown className="w-3 h-3" /> : <IconChevronUp className="w-3 h-3" />) : null}
        </div>
        <div className="col-span-2">Category</div>
        <div className="col-span-2 flex items-center gap-1 cursor-pointer hover:text-foreground" onClick={() => toggleSort('priority')}>
          Score {sortBy === 'priority' ? (sortDir === 'desc' ? <IconChevronDown className="w-3 h-3" /> : <IconChevronUp className="w-3 h-3" />) : null}
        </div>
        <div className="col-span-2">Status</div>
        <div className="col-span-1 flex items-center gap-1 cursor-pointer hover:text-foreground" onClick={() => toggleSort('created_at')}>
          Date {sortBy === 'created_at' ? (sortDir === 'desc' ? <IconChevronDown className="w-3 h-3" /> : <IconChevronUp className="w-3 h-3" />) : null}
        </div>
      </div>

      {/* Rows */}
      {loading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-12 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
      ) : paginated.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-8">No ideas found</p>
      ) : (
        <>
          {paginated.map(idea => (
            <div key={idea.id} className="grid grid-cols-12 gap-2 items-center px-3 py-2.5 rounded-lg border border-border hover:bg-muted/50 transition-colors cursor-pointer group">
              <div className="col-span-5 min-w-0">
                <div className="text-sm font-medium text-foreground truncate">{idea.title}</div>
                <div className="text-xs text-muted-foreground truncate opacity-0 group-hover:opacity-100 transition-opacity">{idea.description}</div>
              </div>
              <div className="col-span-2">
                <span className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium ${CATEGORY_COLORS[idea.category] || 'bg-muted text-muted-foreground'}`}>
                  {idea.category}
                </span>
              </div>
              <div className="col-span-2 text-sm font-mono text-muted-foreground">
                {Math.round(idea.priority * 100)}
              </div>
              <div className="col-span-2 flex items-center gap-1.5">
                {STATUS_ICONS[idea.status] || null}
                <span className="text-xs text-muted-foreground capitalize">{idea.status}</span>
              </div>
              <div className="col-span-1 text-xs text-muted-foreground">
                {new Date(idea.created_at).toLocaleDateString()}
              </div>
            </div>
          ))}

          {hasMore && (
            <button
              onClick={() => setPage(p => p + 1)}
              className="w-full py-2 text-xs text-muted-foreground hover:text-foreground border border-dashed border-border rounded-lg hover:border-primary/50 transition-colors"
            >
              Load more
            </button>
          )}
        </>
      )}
    </div>
  );
}