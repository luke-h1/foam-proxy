import { describe, expect, test } from 'bun:test';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const script = join(import.meta.dir, 'release-notes.ts');

function repo(changesets: Record<string, string>, changelog = '# Changelog\n'): string {
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
  writeFileSync(join(dir, 'CHANGELOG.md'), changelog);
  for (const [id, body] of Object.entries(changesets)) {
    writeFileSync(join(dir, '.changeset', `${id}.md`), body);
  }
  return dir;
}

interface Result {
  exitCode: number;
  stdout: string;
  stderr: string;
}

function spawn(cwd: string, ...args: string[]): Result {
  const result = Bun.spawnSync(['bun', script, ...args], { cwd });
  return {
    exitCode: result.exitCode,
    stdout: result.stdout.toString(),
    stderr: result.stderr.toString(),
  };
}

function run(cwd: string, ...args: string[]): string {
  const result = spawn(cwd, ...args);
  if (result.exitCode !== 0) {
    throw new Error(result.stderr);
  }
  return result.stdout;
}

const fixed = "---\n'foam-proxy': patch\n---\n\nThe healthcheck answers faster.\n";
const added = "---\n'foam-proxy': minor\n---\n\nA new route lists the versions.\n";
const blankMajor = "---\n'foam-proxy': major\n---\n";

const released =
  '# Changelog\n\n## 0.4.0 - 2026-11-17\n\n### New\n\n- A new route lists the versions.\n\n### Fixed\n\n- The healthcheck answers faster.\n';

describe('flags', () => {
  test('a flag with no operand is an error', () => {
    const result = spawn(repo({}), 'next-version', '--current');
    expect(result.exitCode).toBe(1);
    expect(result.stderr.trim()).toBe('--current needs a value');
  });

  test('--level must be a positive integer', () => {
    const result = spawn(repo({ one: fixed }), 'pending', '--level', '0');
    expect(result.exitCode).toBe(1);
    expect(result.stderr.trim()).toBe('--level must be a positive integer');
  });
});

describe('next-version', () => {
  test('applies the highest bump to the current version', () => {
    const dir = repo({ one: fixed, two: added });
    expect(run(dir, 'next-version', '--current', '0.3.0')).toBe('0.4.0\n');
  });

  test('prints nothing when no changeset asks for a bump', () => {
    const dir = repo({ empty: '---\n---\n' });
    expect(run(dir, 'next-version', '--current', '0.3.0')).toBe('');
  });

  test('rejects a bump with no text', () => {
    const result = spawn(repo({ one: fixed, blank: blankMajor }), 'next-version', '--current', '0.3.0');
    expect(result.exitCode).toBe(1);
    expect(result.stderr.trim()).toBe(
      '.changeset/blank.md has a bump but no text; write the note in the file or delete it and run bun run changeset -- --empty',
    );
  });

  test('rejects text with no package and bump', () => {
    const result = spawn(repo({ loose: '---\n---\n\nA note nobody will render.\n' }), 'pending');
    expect(result.exitCode).toBe(1);
    expect(result.stderr.trim()).toBe(
      '.changeset/loose.md has text but no package and bump; add them to the file or delete it and run bun run changeset',
    );
  });
});

describe('release', () => {
  test('writes the section and deletes the changesets', () => {
    const dir = repo({ one: fixed, two: added });
    const out = run(dir, 'release', '--version', '0.4.0', '--date', '2026-11-17');
    const changelog = readFileSync(join(dir, 'CHANGELOG.md'), 'utf8');
    expect(changelog).toBe(released);
    expect(out).toBe(
      '### New\n\n- A new route lists the versions.\n\n### Fixed\n\n- The healthcheck answers faster.\n',
    );
    expect(existsSync(join(dir, '.changeset', 'one.md'))).toBe(false);
    expect(existsSync(join(dir, '.changeset', 'two.md'))).toBe(false);
  });

  test('refuses an option token as a flag value and touches nothing', () => {
    const dir = repo({ one: fixed });
    const result = spawn(dir, 'release', '--version', '--date', '2026-11-17');
    expect(result.exitCode).toBe(1);
    expect(result.stderr.trim()).toBe('--version needs a value');
    expect(readFileSync(join(dir, 'CHANGELOG.md'), 'utf8')).toBe('# Changelog\n');
    expect(existsSync(join(dir, '.changeset', 'one.md'))).toBe(true);
  });

  test('appends a second block of headings to an existing section', () => {
    const dir = repo({ three: "---\n'foam-proxy': patch\n---\n\nThe authorizer logs the consumer.\n" }, released);
    run(dir, 'release', '--version', '0.4.0', '--date', '2026-11-18');
    expect(readFileSync(join(dir, 'CHANGELOG.md'), 'utf8')).toBe(
      `${released.trimEnd()}\n\n### Fixed\n\n- The authorizer logs the consumer.\n\n`,
    );
  });
});

describe('show', () => {
  test('prints the body of a recorded section', () => {
    const dir = repo({}, released);
    expect(run(dir, 'show', '--version', '0.4.0')).toBe(
      '### New\n\n- A new route lists the versions.\n\n### Fixed\n\n- The healthcheck answers faster.\n',
    );
  });

  test('fails for a version with no section', () => {
    const result = spawn(repo({}, released), 'show', '--version', '0.5.0');
    expect(result.exitCode).toBe(1);
    expect(result.stderr.trim()).toBe('CHANGELOG.md has no section for 0.5.0');
  });
});
