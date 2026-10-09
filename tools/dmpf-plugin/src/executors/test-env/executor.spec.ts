import { runScript, scriptPath } from '../../lib/run';
import { executorContext } from '../../testing/executor-context';
import testEnvExecutor from './executor';

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: false })),
}));

describe('[executor] test-env', () => {
  it('should run the command through test-env.sh in the project directory and report its outcome', async () => {
    const command = 'go test -race -count=1 -p 1 -tags=integration ./...';

    const result = await testEnvExecutor({ command }, executorContext('apps/backend/bookings'));

    expect(result).toEqual({ success: false });
    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('test-env.sh'),
      args: ['bash', '-c', command],
      cwd: '/workspace/apps/backend/bookings',
      env: expect.objectContaining({ DMPF_WORKSPACE_ROOT: '/workspace' }),
    });
  });
});
