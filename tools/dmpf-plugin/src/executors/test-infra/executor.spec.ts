import { runScript, scriptPath } from '../../lib/run';
import { executorContext } from '../../testing/executor-context';
import testInfraExecutor from './executor';

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: true })),
}));

describe('[executor] test-infra', () => {
  it.each([
    'up',
    'down',
  ] as const)('should run test-infra.sh %s from the workspace root', async (action) => {
    const result = await testInfraExecutor({ action }, executorContext('libs/backend/go/testkit'));

    expect(result).toEqual({ success: true });
    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('test-infra.sh'),
      args: [action],
      cwd: '/workspace',
      env: expect.objectContaining({ DMPF_WORKSPACE_ROOT: '/workspace' }),
    });
  });
});
