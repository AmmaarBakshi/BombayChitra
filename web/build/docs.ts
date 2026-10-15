// Builds the documentation site from ../docs/*.md and the API reference from ../api/openapi.yaml.
// Pages are emitted as static HTML under /docs/, plus /openapi.json and /openapi.yaml at the root. In dev the same
// pages are rendered on request.
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { Marked, type Tokens } from 'marked';
import type { Plugin } from 'vite';
import { parse as parseYaml } from 'yaml';
import { escapeHtml, highlight } from './highlight.ts';

export interface DocsOptions {
  docsDir: string;
  openapi: string;
  repoUrl: string;
}

interface NavSection {
  title: string;
  pages: string[];
}

interface Page {
  slug: string;
  title: string;
  description: string;
  html: string;
  toc: { id: string; text: string; level: number }[];
}

const here = path.dirname(fileURLToPath(import.meta.url));
const API_SLUG = 'api-reference';

const urlFor = (slug: string) => (slug === 'index' ? '/docs/' : `/docs/${slug}/`);

/** Plain text of rendered inline HTML (tags removed, entities decoded) for titles and the table of contents. */
function plainText(html: string): string {
  return html
    .replace(/<[^>]+>/g, '')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&');
}

function slugify(text: string): string {
  return text.toLowerCase().replace(/<[^>]+>/g, '').replace(/&[a-z]+;/g, '').replace(/[^\w\s-]/g, '').trim().replace(/\s+/g, '-');
}

// ------------------------------------------------------------------ markdown

function renderMarkdown(source: string, repoUrl: string): Omit<Page, 'slug'> {
  const toc: Page['toc'] = [];
  const used = new Map<string, number>();
  let title = '';
  const marked = new Marked({
    gfm: true,
    renderer: {
      heading({ tokens, depth }: Tokens.Heading) {
        const inner = this.parser.parseInline(tokens);
        if (depth === 1 && !title) {
          title = plainText(inner);
          return `<h1>${inner}</h1>\n`;
        }
        let id = slugify(inner);
        const n = used.get(id) ?? 0;
        used.set(id, n + 1);
        if (n) id = `${id}-${n}`;
        if (depth <= 3) toc.push({ id, text: plainText(inner), level: depth });
        return `<h${depth} id="${id}"><a class="anchor" href="#${id}" aria-hidden="true">#</a>${inner}</h${depth}>\n`;
      },
      link({ href, title: linkTitle, tokens }: Tokens.Link) {
        const text = this.parser.parseInline(tokens);
        let url = href;
        const md = href.match(/^(?:\.\/)?([\w-]+)\.md(#.*)?$/);
        if (md) url = urlFor(md[1] === 'README' ? 'index' : md[1]) + (md[2] ?? '');
        else if (href.startsWith('../')) url = `${repoUrl}/blob/main/${href.replace(/^(\.\.\/)+/, '')}`;
        const external = /^https?:\/\//.test(url) && !url.startsWith(repoUrl + '/blob');
        return `<a href="${escapeHtml(url)}"${linkTitle ? ` title="${escapeHtml(linkTitle)}"` : ''}${external ? ' target="_blank" rel="noopener"' : ''}>${text}</a>`;
      },
      code({ text, lang }: Tokens.Code) {
        const language = (lang ?? '').split(/\s+/)[0];
        const label = language ? `<span class="code-lang">${escapeHtml(language)}</span>` : '';
        return `<div class="code">${label}<button class="copy" type="button" aria-label="Copy code">Copy</button><pre><code class="language-${escapeHtml(language)}">${highlight(text, language)}</code></pre></div>\n`;
      },
      table(token: Tokens.Table) {
        const head = token.header.map((c) => `<th${c.align ? ` style="text-align:${c.align}"` : ''}>${this.parser.parseInline(c.tokens)}</th>`).join('');
        const rows = token.rows.map((r) => `<tr>${r.map((c) => `<td${c.align ? ` style="text-align:${c.align}"` : ''}>${this.parser.parseInline(c.tokens)}</td>`).join('')}</tr>`).join('');
        return `<div class="table"><table><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>\n`;
      },
    },
  });
  const html = marked.parse(source) as string;
  const firstPara = source.split('\n').find((l) => l.trim() && !l.startsWith('#') && !l.startsWith('>') && !l.startsWith('|') && !l.startsWith('```')) ?? '';
  const description = firstPara.replace(/[*_`[\]]/g, '').replace(/\(([^)]+)\)/g, '').slice(0, 160);
  return { title: title || 'BombayChitra', description, html, toc };
}

// ------------------------------------------------------------------ OpenAPI reference

interface OpenApiParam {
  name: string;
  in: string;
  required?: boolean;
  description?: string;
  schema?: { type?: string; enum?: string[]; default?: unknown; minimum?: number; maximum?: number };
  example?: unknown;
}

interface OpenApiOperation {
  operationId?: string;
  summary?: string;
  description?: string;
  tags?: string[];
  parameters?: (OpenApiParam | { $ref: string })[];
  requestBody?: { content?: Record<string, { example?: unknown; schema?: BodySchema }> };
  responses?: Record<string, { description?: string; content?: Record<string, { example?: unknown }> }>;
}

/** The parts of a request body schema the reference shows: its fields, their types and limits. */
interface BodyField {
  type?: string;
  enum?: string[];
  default?: unknown;
  minimum?: number;
  maximum?: number;
  minItems?: number;
  maxItems?: number;
  items?: BodyField;
  $ref?: string;
}

interface BodySchema {
  required?: string[];
  properties?: Record<string, BodyField>;
}

interface OpenApiDoc {
  info: { title: string; version: string; description?: string };
  servers?: { url: string }[];
  tags?: { name: string; description?: string }[];
  paths: Record<string, Record<string, OpenApiOperation>>;
  components?: { parameters?: Record<string, OpenApiParam> };
}

function renderApiReference(spec: OpenApiDoc): Omit<Page, 'slug'> {
  const toc: Page['toc'] = [];
  const resolve = (p: OpenApiParam | { $ref: string }): OpenApiParam => {
    if ('$ref' in p) {
      const { $ref, ...overrides } = p as { $ref: string } & Partial<OpenApiParam>;
      const key = $ref.split('/').pop()!;
      return { ...(spec.components?.parameters?.[key] ?? { name: key, in: 'query' }), ...overrides };
    }
    return p;
  };
  const md = new Marked({ gfm: true });
  const out: string[] = [];
  out.push(`<h1>API reference</h1>`);
  out.push(md.parse(spec.info.description ?? '') as string);
  const ops: { tag: string; method: string; path: string; op: OpenApiOperation }[] = [];
  for (const [p, methods] of Object.entries(spec.paths)) {
    for (const [method, op] of Object.entries(methods)) ops.push({ tag: op.tags?.[0] ?? 'Other', method, path: p, op });
  }
  for (const tag of spec.tags ?? []) {
    const id = slugify(tag.name);
    toc.push({ id, text: tag.name, level: 2 });
    out.push(`<h2 id="${id}"><a class="anchor" href="#${id}" aria-hidden="true">#</a>${escapeHtml(tag.name)}</h2>`);
    if (tag.description) out.push(md.parse(tag.description) as string);
    for (const { method, path: p, op } of ops.filter((o) => o.tag === tag.name)) {
      const opId = op.operationId ?? slugify(`${method} ${p}`);
      toc.push({ id: opId, text: op.summary ?? p, level: 3 });
      const params = (op.parameters ?? []).map(resolve);
      const exampleQuery = params.filter((x) => x.in === 'query' && x.example !== undefined).map((x) => `${x.name}=${encodeURIComponent(String(x.example))}`).join('&');
      let examplePath = p;
      for (const x of params.filter((q) => q.in === 'path')) examplePath = examplePath.replace(`{${x.name}}`, String(x.example ?? `{${x.name}}`));
      const body = op.requestBody?.content && Object.values(op.requestBody.content)[0];
      const curl = body
        ? `curl -X ${method.toUpperCase()} "http://localhost:8080${examplePath}" \\\n  -H 'Content-Type: application/json' \\\n  -d '${JSON.stringify(body.example ?? {})}'`
        : `curl "http://localhost:8080${examplePath}${exampleQuery ? `?${exampleQuery}` : ''}"`;
      const ok = op.responses?.['200'];
      const example = ok?.content && Object.values(ok.content)[0]?.example;
      out.push(`<section class="endpoint" id="${opId}">`);
      out.push(`<div class="ep-head"><span class="method ${method}">${method.toUpperCase()}</span><code>${escapeHtml(p)}</code></div>`);
      if (op.summary) out.push(`<h3>${escapeHtml(op.summary)}</h3>`);
      if (op.description) out.push(md.parse(op.description) as string);
      if (params.length) {
        out.push('<div class="table"><table><thead><tr><th>Parameter</th><th>In</th><th>Type</th><th>Description</th></tr></thead><tbody>');
        for (const x of params) {
          const s = x.schema ?? {};
          const type = s.enum ? s.enum.map((e) => `<code>${escapeHtml(e)}</code>`).join(' | ') : escapeHtml(s.type ?? 'string');
          const bounds = [s.default !== undefined ? `default <code>${escapeHtml(String(s.default))}</code>` : '', s.maximum !== undefined ? `max ${s.maximum}` : ''].filter(Boolean).join(', ');
          out.push(`<tr><td><code>${escapeHtml(x.name)}</code>${x.required ? ' <span class="req">required</span>' : ''}</td><td>${x.in}</td><td>${type}</td><td>${md.parseInline(x.description ?? '') as string}${bounds ? ` <span class="muted">(${bounds})</span>` : ''}</td></tr>`);
        }
        out.push('</tbody></table></div>');
      }
      if (body?.schema?.properties) {
        const typeOf = (f: BodyField): string => {
          if (f.$ref) return escapeHtml(f.$ref.split('/').pop()!);
          if (f.enum) return f.enum.map((e) => `<code>${escapeHtml(e)}</code>`).join(' | ');
          if (f.type === 'array' && f.items) return `array of ${typeOf(f.items)}`;
          return escapeHtml(f.type ?? 'object');
        };
        out.push('<div class="table"><table><thead><tr><th>Body field</th><th>Type</th><th>Limits</th></tr></thead><tbody>');
        for (const [name, f] of Object.entries(body.schema.properties)) {
          const limits = [
            f.default !== undefined ? `default <code>${escapeHtml(String(f.default))}</code>` : '',
            f.minimum !== undefined ? `min ${f.minimum}` : '',
            f.maximum !== undefined ? `max ${f.maximum}` : '',
            f.minItems !== undefined ? `at least ${f.minItems}` : '',
            f.maxItems !== undefined ? `at most ${f.maxItems}` : '',
          ].filter(Boolean).join(', ');
          out.push(`<tr><td><code>${escapeHtml(name)}</code>${body.schema.required?.includes(name) ? ' <span class="req">required</span>' : ''}</td><td>${typeOf(f)}</td><td>${limits}</td></tr>`);
        }
        out.push('</tbody></table></div>');
      }
      out.push(`<div class="code"><span class="code-lang">bash</span><button class="copy" type="button" aria-label="Copy code">Copy</button><pre><code>${highlight(curl, 'bash')}</code></pre></div>`);
      if (example !== undefined) {
        out.push(`<details class="example"><summary>Example response</summary><div class="code"><span class="code-lang">json</span><pre><code>${highlight(JSON.stringify(example, null, 2), 'json')}</code></pre></div></details>`);
      }
      out.push('</section>');
    }
  }
  return { title: 'API reference', description: `${spec.info.title} ${spec.info.version} — every endpoint, parameter and response.`, html: out.join('\n'), toc };
}

// ------------------------------------------------------------------ site

class DocsSite {
  private nav: NavSection[] = [];
  private pages = new Map<string, Page>();
  private spec: OpenApiDoc | null = null;
  private specRaw = '';

  constructor(private readonly opts: DocsOptions) {}

  load(): void {
    const navPath = path.join(this.opts.docsDir, 'nav.json');
    this.nav = JSON.parse(readFileSync(navPath, 'utf8')).sections as NavSection[];
    this.pages.clear();
    for (const section of this.nav) {
      for (const slug of section.pages) {
        if (slug === API_SLUG) continue;
        const file = path.join(this.opts.docsDir, `${slug}.md`);
        if (!existsSync(file)) throw new Error(`docs/nav.json lists ${slug} but docs/${slug}.md is missing`);
        this.pages.set(slug, { slug, ...renderMarkdown(readFileSync(file, 'utf8'), this.opts.repoUrl) });
      }
    }
    this.specRaw = readFileSync(this.opts.openapi, 'utf8');
    this.spec = parseYaml(this.specRaw) as OpenApiDoc;
    this.pages.set(API_SLUG, { slug: API_SLUG, ...renderApiReference(this.spec) });
  }

  files(): Record<string, string> {
    const files: Record<string, string> = {
      'docs/docs.css': readFileSync(path.join(here, 'docs.css'), 'utf8'),
      'docs/docs.js': readFileSync(path.join(here, 'docs-client.js'), 'utf8'),
      'openapi.yaml': this.specRaw,
      'openapi.json': JSON.stringify(this.spec, null, 2),
    };
    for (const page of this.pages.values()) {
      files[page.slug === 'index' ? 'docs/index.html' : `docs/${page.slug}/index.html`] = this.render(page);
    }
    return files;
  }

  render(page: Page): string {
    const order = this.nav.flatMap((s) => s.pages);
    const i = order.indexOf(page.slug);
    const prev = i > 0 ? this.pages.get(order[i - 1]) : undefined;
    const next = i >= 0 && i < order.length - 1 ? this.pages.get(order[i + 1]) : undefined;
    const nav = this.nav.map((s) => `<div class="nav-section"><div class="nav-title">${escapeHtml(s.title)}</div>${s.pages.map((slug) => {
      const p = this.pages.get(slug);
      return p ? `<a href="${urlFor(slug)}"${slug === page.slug ? ' aria-current="page"' : ''}>${escapeHtml(p.title)}</a>` : '';
    }).join('')}</div>`).join('');
    const toc = page.toc.filter((t) => t.level <= 3).map((t) => `<a class="toc-${t.level}" href="#${t.id}">${escapeHtml(t.text)}</a>`).join('');
    const pager = `<nav class="pager">${prev ? `<a class="prev" href="${urlFor(prev.slug)}"><span>Previous</span>${escapeHtml(prev.title)}</a>` : '<span></span>'}${next ? `<a class="next" href="${urlFor(next.slug)}"><span>Next</span>${escapeHtml(next.title)}</a>` : ''}</nav>`;
    const editUrl = page.slug === API_SLUG ? `${this.opts.repoUrl}/blob/main/api/openapi.yaml` : `${this.opts.repoUrl}/blob/main/docs/${page.slug}.md`;
    return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escapeHtml(page.title)} · BombayChitra docs</title>
<meta name="description" content="${escapeHtml(page.description)}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="stylesheet" href="/docs/docs.css">
<script>(function(){var t='system';try{t=localStorage.getItem('bc-theme')||'system'}catch(e){}var d=t==='dark'||(t==='system'&&matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.dataset.theme=d?'dark':'light'})()</script>
</head>
<body>
<header class="top">
  <button class="menu" type="button" aria-label="Open navigation" aria-expanded="false">☰</button>
  <a class="brand" href="/docs/"><svg width="24" height="24" viewBox="0 0 32 32" aria-hidden="true"><rect width="32" height="32" rx="8" fill="#e8590c"/><path d="M7 22c3-1 5-6 9-6s5 4 9 3M9 11l7-4 7 4M16 7v9" stroke="#fff" stroke-width="2.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>BombayChitra <span>docs</span></a>
  <nav class="top-links"><a href="/">Open map</a><a href="${urlFor(API_SLUG)}">API</a><a href="${this.opts.repoUrl}" target="_blank" rel="noopener">GitHub</a><button class="theme" type="button" aria-label="Toggle theme">◐</button></nav>
</header>
<div class="layout">
  <aside class="sidebar" aria-label="Documentation">${nav}</aside>
  <main class="content">
    <article class="prose">${page.html}</article>
    <p class="edit"><a href="${editUrl}" target="_blank" rel="noopener">Edit this page on GitHub</a></p>
    ${pager}
  </main>
  <aside class="toc" aria-label="On this page">${toc ? `<div class="toc-title">On this page</div>${toc}` : ''}</aside>
</div>
<script src="/docs/docs.js" defer></script>
</body>
</html>`;
  }

  /** Resolves a dev-server URL to a generated file. */
  lookup(url: string): { body: string; type: string } | null {
    const clean = url.split(/[?#]/)[0];
    const files = this.files();
    const key = clean === '/docs' || clean === '/docs/' ? 'docs/index.html'
      : clean.endsWith('/') ? `${clean.slice(1)}index.html`
      : files[clean.slice(1)] !== undefined ? clean.slice(1)
      : `${clean.slice(1)}/index.html`;
    const body = files[key];
    if (body === undefined) return null;
    const ext = path.extname(key);
    const type = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript', '.json': 'application/json', '.yaml': 'application/yaml' }[ext] ?? 'text/plain';
    return { body, type: `${type}; charset=utf-8` };
  }
}

export function docsPlugin(opts: DocsOptions): Plugin {
  const site = new DocsSite(opts);
  return {
    name: 'bombaychitra-docs',
    buildStart() {
      site.load();
      this.addWatchFile(opts.openapi);
      this.addWatchFile(path.join(opts.docsDir, 'nav.json'));
    },
    generateBundle() {
      for (const [fileName, source] of Object.entries(site.files())) this.emitFile({ type: 'asset', fileName, source });
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = req.url ?? '';
        if (!url.startsWith('/docs') && !url.startsWith('/openapi.')) return next();
        try {
          site.load();
          const hit = site.lookup(url);
          if (!hit) return next();
          res.setHeader('Content-Type', hit.type);
          res.end(hit.body);
        } catch (err) {
          next(err);
        }
      });
    },
  };
}
