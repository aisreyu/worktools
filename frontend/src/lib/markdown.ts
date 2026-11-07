// Markdown rendering + export helpers for the Notes page.
import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({
  gfm: true,
  breaks: true,
})

/** Render markdown to sanitized HTML. */
export function renderMarkdown(md: string): string {
  const raw = marked.parse(md ?? '', { async: false }) as string
  return DOMPurify.sanitize(raw, {
    ADD_ATTR: ['target', 'rel'],
  })
}

/** Wrap rendered HTML in a full standalone document with print styles. */
export function wrapHtmlDocument(title: string, bodyHtml: string): string {
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>${escapeHtml(title)}</title>
<style>
  body {
    font-family: -apple-system, "PingFang SC", "Helvetica Neue", Arial, sans-serif;
    max-width: 820px;
    margin: 40px auto;
    padding: 0 24px;
    color: #1f2328;
    line-height: 1.7;
  }
  h1, h2 { border-bottom: 1px solid #d8dee4; padding-bottom: 8px; }
  code {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    background: #f0f1f3;
    padding: 2px 5px;
    border-radius: 4px;
    font-size: 0.9em;
  }
  pre {
    background: #f6f8fa;
    padding: 14px;
    border-radius: 8px;
    overflow-x: auto;
  }
  pre code { background: none; padding: 0; }
  blockquote {
    border-left: 4px solid #d0d7de;
    margin: 0;
    padding: 4px 16px;
    color: #57606a;
    background: #f6f8fa;
  }
  table { border-collapse: collapse; }
  th, td { border: 1px solid #d8dee4; padding: 6px 12px; }
  th { background: #f6f8fa; }
  img { max-width: 100%; }
  a { color: #0969da; }
  @media print {
    body { margin: 20mm; max-width: none; }
    pre, blockquote, table { page-break-inside: avoid; }
  }
</style>
</head>
<body>
<h1>${escapeHtml(title)}</h1>
${bodyHtml}
</body>
</html>`
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}
