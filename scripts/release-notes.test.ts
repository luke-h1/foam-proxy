import { describe, expect, test } from 'bun:test';
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const script = join(import.meta.dir, 'release-notes.ts');

function repo(changesets: Record<string, string>): string {
  const dir = mkdtempSync(join(tmpdir(), 'release-notes-'));
  mkdirSync(join(dir, '.changeset'));
  writeFileSync(
    join(dir, 'package.json'),
    JSON.stringify({ name: 'foam-proxy', private: true, version: '0.0.0' }),
  );
  writeFileSync(
    join(dir, '.changeset', 'config.json'),
    JSON.stringify({ baseBranch: 'main', changelog: false }),
  );
  writeFileSync(join(dir, 'CHANGELOG.md'), '# Changelog\n');
  for (const [id, body] of Object.entries(changesets)) {
    writeFileSync(join(dir, '.changeset', `${id}.md`), body);
  }
  return dir;
}

function run(cwd: string, ...args: string[]): string {
  const result = Bun.spawnSync(['bun', script, ...args], { cwd });
  if (result.exitCode !== 0) {
    throw new Error(result.stderr.toString());
  }
  return result.stdout.toString();
}

const fixed = "---\n'foam-proxy': patch\n---\n\nThe healthcheck answers faster.\n";
const added = "---\n'foam-proxy': minor\n---\n\nA new route lists the versions.\n";

describe('next-version', () => {
  test('applies the highest bump to the current version', () => {
    const dir = repo({ one: fixed, two: added });
    expect(run(dir, 'next-version', '--current', '0.3.0')).toBe('0.4.0\n');
  });

  test('prints nothing when no changeset asks for a bump', () => {
    const dir = repo({ empty: '---\n---\n' });
    expect(run(dir, 'next-version', '--current', '0.3.0')).toBe('');
  });
});

describe('release', () => {
  test('writes the section and deletes the changesets', () => {
    const dir = repo({ one: fixed, two: added });
    const out = run(dir, 'release', '--version', '0.4.0', '--date', '2026-11-17');
    const changelog = readFileSync(join(dir, 'CHANGELOG.md'), 'utf8');
    expect(changelog).toBe(
      '# Changelog\n\n## 0.4.0 - 2026-11-17\n\n### New\n\n- A new route lists the versions.\n\n### Fixed\n\n- The healthcheck answers faster.\n',
    );
    expect(out.trim()).toBe(changelog.slice(changelog.indexOf('## 0.4.0')).trim());
    expect(existsSync(join(dir, '.changeset', 'one.md'))).toBe(false);
    expect(existsSync(join(dir, '.changeset', 'two.md'))).toBe(false);
  });
});
