'use client';

import { useState } from 'react';
import { IconFileText, IconCopy, IconCheck } from '@tabler/icons-react';

interface ResearchReportProps {
  reportContent?: string;
  phase?: string;
  cycleId?: string;
}

// Simple structured research report display
export function ResearchReport({ reportContent, phase, cycleId }: ResearchReportProps) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    if (!reportContent) return;
    await navigator.clipboard.writeText(reportContent);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  if (!reportContent) {
    return (
      <div className="text-center py-12">
        <IconFileText className="w-10 h-10 text-muted-foreground mx-auto mb-3" />
        <p className="text-sm text-muted-foreground">No research report available</p>
        <p className="text-xs text-muted-foreground mt-1">Run a research cycle to generate a report</p>
      </div>
    );
  }

  // Try to parse into sections
  const sections = parseReportSections(reportContent);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          {phase && <span className="text-xs px-2 py-0.5 rounded bg-primary/10 text-primary">{phase}</span>}
          {cycleId && <span className="text-xs text-muted-foreground ml-2">Cycle #{cycleId.slice(0, 8)}</span>}
        </div>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-border hover:bg-muted text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {copied ? <IconCheck className="w-3.5 h-3.5 text-green-400" /> : <IconCopy className="w-3.5 h-3.5" />}
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      {/* Sections */}
      {sections.map((section, i) => (
        <div key={i} className="space-y-2">
          <h3 className="text-sm font-semibold text-foreground border-b border-border pb-1">{section.title}</h3>
          <div className="text-sm text-muted-foreground leading-relaxed whitespace-pre-wrap">{section.content}</div>
        </div>
      ))}

      {/* Raw content if no sections detected */}
      {sections.length === 0 && (
        <div className="text-sm text-muted-foreground leading-relaxed whitespace-pre-wrap">{reportContent}</div>
      )}
    </div>
  );
}

function parseReportSections(content: string): Array<{ title: string; content: string }> {
  // Look for markdown headers (## Title) or numbered sections
  const lines = content.split('\n');
  const sections: Array<{ title: string; content: string }> = [];
  let currentTitle = '';
  let currentContent: string[] = [];

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('## ') || trimmed.match(/^\d+\.\s+\w/)) {
      if (currentTitle) {
        sections.push({ title: currentTitle, content: currentContent.join('\n').trim() });
      }
      currentTitle = trimmed.replace(/^##\s*/, '').replace(/^\d+\.\s*/, '');
      currentContent = [];
    } else {
      currentContent.push(trimmed);
    }
  }

  if (currentTitle) {
    sections.push({ title: currentTitle, content: currentContent.join('\n').trim() });
  }

  return sections;
}