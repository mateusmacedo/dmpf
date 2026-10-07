import { runScript, scriptPath } from '../../lib/run';
import { readVersions } from '../../lib/versions';
import { executorContext } from '../../testing/executor-context';
import bufExecutor from './executor';

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: true })),
}));

describe('[executor] buf', () => {
  it('should run the Buf CLI pinned by versions.json in the contract module', async () => {
    const result = await bufExecutor(
      { args: ['generate'] },
      executorContext('apps/backend/bookings/contract'),
    );

    expect(result).toEqual({ success: true });
    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('buf.sh'),
      args: ['generate'],
      cwd: '/workspace/apps/backend/bookings/contract',
      env: expect.objectContaining({ DMPF_BUF_VERSION: readVersions().buf }),
    });
  });
});
