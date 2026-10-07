import { execFileSync } from 'node:child_process';
import { runScript, scriptPath } from '../../lib/run';
import { executorContext } from '../../testing/executor-context';
import { workspaceWithDmpfJson } from '../../testing/workspace';
import contextCheckExecutor, { KERNEL_POSTGRES_MODULE } from './executor';

jest.mock('node:child_process', () => ({
  ...jest.requireActual<typeof import('node:child_process')>('node:child_process'),
  execFileSync: jest.fn(() => JSON.stringify({ Dir: '/gomodcache/postgres@v1.0.0' })),
}));

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: true })),
}));

beforeEach(() => {
  jest.mocked(runScript).mockClear();
  jest.mocked(execFileSync).mockClear();
});

describe('[executor] context-check', () => {
  it('should check the workspace with the kernel DDL of the workspace in local mode', async () => {
    const root = workspaceWithDmpfJson('local');

    await contextCheckExecutor({}, executorContext('apps/backend/bookings', root));

    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('dmpf-context-check.sh'),
      args: ['--phase', 'structural'],
      cwd: root,
      env: expect.objectContaining({
        DMPF_APPS_DIR: 'apps/backend',
        DMPF_KERNEL_DDL: 'libs/backend/go/postgres',
      }),
    });
    expect(execFileSync).not.toHaveBeenCalled();
  });

  it('should take the kernel DDL from the module cache in version mode', async () => {
    const root = workspaceWithDmpfJson('version');

    await contextCheckExecutor(
      { context: 'apps/backend/orders' },
      executorContext('apps/backend/orders', root),
    );

    expect(execFileSync).toHaveBeenCalledWith(
      'go',
      ['mod', 'download', '-json', KERNEL_POSTGRES_MODULE],
      {
        cwd: root,
        encoding: 'utf-8',
      },
    );
    expect(jest.mocked(runScript).mock.calls[0][0]).toMatchObject({
      args: ['--phase', 'structural', '--context', 'apps/backend/orders'],
      env: { DMPF_KERNEL_DDL: '/gomodcache/postgres@v1.0.0' },
    });
  });

  it('should refuse the self-test outside the platform, before running anything', async () => {
    const root = workspaceWithDmpfJson('version');

    const result = await contextCheckExecutor(
      { phase: 'self-test' },
      executorContext('apps/backend/orders', root),
    );

    expect(result).toEqual({ success: false });
    expect(runScript).not.toHaveBeenCalled();
  });
});
