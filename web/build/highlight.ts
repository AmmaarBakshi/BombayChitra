// A tiny, dependency-free syntax highlighter for the documentation's code samples. It recognises comments, strings,
// numbers and keywords — enough to make samples in a dozen languages readable without shipping a highlighter.

const KEYWORDS: Record<string, string[]> = {
  js: ['const', 'let', 'var', 'function', 'return', 'if', 'else', 'for', 'of', 'in', 'await', 'async', 'import', 'from', 'export', 'new', 'class', 'try', 'catch', 'throw', 'true', 'false', 'null', 'undefined', 'typeof', 'interface', 'type'],
  python: ['import', 'from', 'as', 'def', 'return', 'if', 'elif', 'else', 'for', 'in', 'with', 'print', 'True', 'False', 'None', 'and', 'or', 'not', 'class', 'try', 'except', 'lambda', 'async', 'await'],
  go: ['package', 'import', 'func', 'return', 'if', 'else', 'for', 'range', 'var', 'const', 'type', 'struct', 'defer', 'go', 'nil', 'err', 'true', 'false', 'map', 'string', 'int', 'float64', 'any'],
  java: ['import', 'public', 'class', 'static', 'void', 'var', 'new', 'return', 'try', 'catch', 'throws', 'final', 'String', 'if', 'else', 'for', 'record', 'true', 'false', 'null'],
  rust: ['use', 'fn', 'let', 'mut', 'async', 'await', 'pub', 'struct', 'impl', 'return', 'match', 'Ok', 'Err', 'Some', 'None', 'for', 'in', 'if', 'else', 'true', 'false'],
  csharp: ['using', 'var', 'new', 'await', 'async', 'class', 'public', 'static', 'void', 'string', 'return', 'foreach', 'in', 'if', 'else', 'true', 'false', 'null', 'record'],
  php: ['<?php', 'echo', 'foreach', 'as', 'function', 'return', 'true', 'false', 'null', 'new', 'if', 'else'],
  ruby: ['require', 'def', 'end', 'do', 'puts', 'each', 'if', 'else', 'true', 'false', 'nil'],
  sql: ['SELECT', 'FROM', 'WHERE', 'JOIN', 'ON', 'AND', 'OR', 'ORDER', 'BY', 'GROUP', 'LIMIT', 'AS', 'IN', 'NOT', 'NULL', 'IS', 'DESC', 'ASC', 'COUNT', 'MATCH', 'LIKE', 'INNER', 'LEFT', 'BETWEEN', 'WITH', 'CREATE', 'TABLE', 'INDEX', 'VIRTUAL', 'USING', 'PRIMARY', 'KEY', 'INTEGER', 'TEXT', 'REAL'],
  bash: ['curl', 'cd', 'git', 'docker', 'npm', 'go', 'java', 'export', 'echo', 'make', 'sudo'],
  kotlin: ['val', 'var', 'fun', 'import', 'class', 'return', 'if', 'else', 'true', 'false', 'null'],
  swift: ['import', 'let', 'var', 'func', 'return', 'if', 'else', 'true', 'false', 'nil', 'class', 'struct'],
  dart: ['import', 'final', 'var', 'void', 'return', 'await', 'async', 'class', 'true', 'false', 'null'],
};
const ALIASES: Record<string, string> = {
  javascript: 'js', ts: 'js', typescript: 'js', mjs: 'js', html: 'js', py: 'python', sh: 'bash', shell: 'bash', console: 'bash',
  powershell: 'bash', ps1: 'bash', cs: 'csharp', 'c#': 'csharp', rs: 'rust', kt: 'kotlin', sqlite: 'sql', golang: 'go',
};
const HASH_COMMENT = new Set(['python', 'bash', 'ruby', 'yaml', 'toml', 'php']);
const DASH_COMMENT = new Set(['sql']);

export function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

export function highlight(code: string, langRaw = ''): string {
  const lang = ALIASES[langRaw.toLowerCase()] ?? langRaw.toLowerCase();
  const keywords = new Set(KEYWORDS[lang] ?? []);
  const ci = lang === 'sql';
  const parts: string[] = [];
  let i = 0;
  const push = (cls: string | null, text: string) => parts.push(cls ? `<span class="tk-${cls}">${escapeHtml(text)}</span>` : escapeHtml(text));
  while (i < code.length) {
    const rest = code.slice(i);
    let m: RegExpMatchArray | null;
    if ((HASH_COMMENT.has(lang) && (m = rest.match(/^#[^\n]*/))) ||
      (DASH_COMMENT.has(lang) && (m = rest.match(/^--[^\n]*/))) ||
      (!HASH_COMMENT.has(lang) && lang !== 'json' && (m = rest.match(/^(\/\/[^\n]*|\/\*[\s\S]*?\*\/)/)))) {
      push('c', m[0]);
    } else if ((m = rest.match(/^("(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`)/))) {
      const isKey = lang === 'json' && /^\s*:/.test(code.slice(i + m[0].length));
      push(isKey ? 'p' : 's', m[0]);
    } else if ((m = rest.match(/^-?\b\d+(\.\d+)?([eE][+-]?\d+)?\b/))) {
      push('n', m[0]);
    } else if (lang === 'yaml' && (m = rest.match(/^[\w-]+(?=:)/))) {
      push('p', m[0]);
    } else if ((m = rest.match(/^[A-Za-z_$<?][\w$?]*/))) {
      const word = m[0];
      push(keywords.has(ci ? word.toUpperCase() : word) ? 'k' : null, word);
    } else {
      push(null, code[i]);
      i++;
      continue;
    }
    i += m[0].length;
  }
  return parts.join('');
}
