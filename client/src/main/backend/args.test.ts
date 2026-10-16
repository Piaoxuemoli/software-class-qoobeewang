// @vitest-environment node
import { describe, expect, it } from 'vitest';
import type { BackupRequest } from '../../shared/api';
import {
  backupArgs,
  configGetArgs,
  configSetArgs,
  deleteArgs,
  dryRunArgs,
  filterArgs,
  infoArgs,
  listArgs,
  restoreArgs,
  versionArgs,
} from './args';

const base: BackupRequest = {
  source: '/home/a/docs',
  filters: {},
  compress: false,
  encrypt: false,
};

describe('filterArgs (contract §2.1)', () => {
  it('emits nothing for empty filters', () => {
    expect(filterArgs({})).toEqual([]);
  });

  it('joins --types with commas', () => {
    expect(filterArgs({ types: ['f', 'd', 'l'] })).toEqual(['--types', 'f,d,l']);
  });

  it('passes an empty type list through for the backend to reject', () => {
    expect(filterArgs({ types: [] })).toEqual(['--types', '']);
  });

  it('repeats repeatable flags once per value', () => {
    expect(
      filterArgs({
        excludeDirs: ['node_modules', '/home/a/docs/.git'],
        excludeNames: ['\\.tmp$', '-bak$'],
        excludeUsers: ['alice', '1001'],
        excludeGroups: ['staff'],
      }),
    ).toEqual([
      '--exclude-dir',
      'node_modules',
      '--exclude-dir',
      '/home/a/docs/.git',
      '--exclude-name',
      '\\.tmp$',
      '--exclude-name',
      '-bak$',
      '--exclude-user',
      'alice',
      '--exclude-user',
      '1001',
      '--exclude-group',
      'staff',
    ]);
  });

  it('maps time and size intervals', () => {
    expect(
      filterArgs({
        mtimeFrom: '2026-01-01',
        mtimeTo: '2026-10-15T23:59:59',
        ctimeFrom: '2025-01-01',
        ctimeTo: '2025-12-31',
        sizeMin: 0,
        sizeMax: 1048576,
      }),
    ).toEqual([
      '--mtime-from',
      '2026-01-01',
      '--mtime-to',
      '2026-10-15T23:59:59',
      '--ctime-from',
      '2025-01-01',
      '--ctime-to',
      '2025-12-31',
      '--size-min',
      '0',
      '--size-max',
      '1048576',
    ]);
  });

  it('emits only the bounds that are set', () => {
    expect(filterArgs({ mtimeTo: '2026-10-15', sizeMin: 10 })).toEqual([
      '--mtime-to',
      '2026-10-15',
      '--size-min',
      '10',
    ]);
  });
});

describe('backupArgs', () => {
  it('maps the minimal request', () => {
    expect(backupArgs(base)).toEqual(['backup', '--source', '/home/a/docs']);
  });

  it('maps name, filters, huffman compression and aes256 encryption', () => {
    expect(
      backupArgs({
        ...base,
        name: 'docs-1',
        filters: { types: ['f'], excludeDirs: ['tmp'] },
        compress: true,
        encrypt: true,
      }),
    ).toEqual([
      'backup',
      '--source',
      '/home/a/docs',
      '--name',
      'docs-1',
      '--types',
      'f',
      '--exclude-dir',
      'tmp',
      '--compress',
      'huffman',
      '--encrypt',
      'aes256',
      '--password-stdin',
    ]);
  });

  it('keeps values that look like flags as separate argument values', () => {
    expect(backupArgs({ ...base, source: '--help' })).toEqual(['backup', '--source', '--help']);
  });
});

describe('dryRunArgs', () => {
  it('adds --dry-run and drops name/compress/encrypt (no password needed)', () => {
    expect(
      dryRunArgs({
        ...base,
        name: 'n',
        compress: true,
        encrypt: true,
        filters: { sizeMax: 5 },
      }),
    ).toEqual(['backup', '--dry-run', '--source', '/home/a/docs', '--size-max', '5']);
  });
});

describe('restoreArgs', () => {
  it('maps backup and target', () => {
    expect(restoreArgs({ backup: 'docs-1', target: '/tmp/r' }, false)).toEqual([
      'restore',
      '--backup',
      'docs-1',
      '--target',
      '/tmp/r',
    ]);
  });

  it('adds --password-stdin when a password is supplied', () => {
    expect(restoreArgs({ backup: '/x/a.dbk', target: '/tmp/r' }, true)).toEqual([
      'restore',
      '--backup',
      '/x/a.dbk',
      '--target',
      '/tmp/r',
      '--password-stdin',
    ]);
  });
});

describe('query commands', () => {
  it('maps version/list/info/delete/config', () => {
    expect(versionArgs()).toEqual(['--version']);
    expect(listArgs()).toEqual(['list']);
    expect(infoArgs('docs-1')).toEqual(['info', 'docs-1']);
    expect(deleteArgs('docs-1')).toEqual(['delete', 'docs-1']);
    expect(configGetArgs()).toEqual(['config', 'get']);
    expect(configSetArgs('repo', '/data/repo')).toEqual(['config', 'set', 'repo', '/data/repo']);
  });
});
