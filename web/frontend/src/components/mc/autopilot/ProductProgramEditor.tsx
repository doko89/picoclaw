'use client';

import { useState } from 'react';
import { IconDeviceFloppy, IconEye } from '@tabler/icons-react';

interface ProductProgramEditorProps {
  initialContent?: string;
  onSave?: (content: string) => void;
}

export function ProductProgramEditor({ initialContent = '', onSave }: ProductProgramEditorProps) {
  const [content, setContent] = useState(initialContent);
  const [preview, setPreview] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  async function handleSave() {
    setSaving(true);
    try {
      onSave?.(content);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-3">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            {content.length} characters
          </span>
          <span className="text-xs text-muted-foreground">·</span>
          <span className="text-xs text-muted-foreground">
            {content.split('\n').length} lines
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setPreview(p => !p)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-medium transition-colors ${
              preview ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background text-muted-foreground hover:text-foreground'
            }`}
          >
            <IconEye className="w-3.5 h-3.5" />
            {preview ? 'Edit' : 'Preview'}
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
              saved ? 'bg-green-500/20 text-green-400' : 'bg-primary text-primary-foreground hover:bg-primary/90'
            } disabled:opacity-50`}
          >
            <IconDeviceFloppy className="w-3.5 h-3.5" />
            {saved ? 'Saved' : saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>

      {/* Editor / Preview */}
      {preview ? (
        <div className="min-h-[300px] p-4 rounded-lg border border-border bg-background prose prose-sm prose-invert max-w-none">
          <pre className="whitespace-pre-wrap text-sm text-foreground">{content || 'No content yet.'}</pre>
        </div>
      ) : (
        <textarea
          value={content}
          onChange={e => setContent(e.target.value)}
          className="min-h-[300px] w-full p-4 rounded-lg border border-border bg-background text-sm text-foreground font-mono resize-y focus:outline-none focus:ring-2 focus:ring-primary/50"
          placeholder="Write your product program in Markdown..."
          spellCheck={false}
        />
      )}

      {/* Tips */}
      <p className="text-xs text-muted-foreground">
        Use Markdown for formatting. This content is used as the system prompt for ideation cycles.
      </p>
    </div>
  );
}