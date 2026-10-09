import { runScript, scriptPath } from '../../lib/run';
import { executorContext } from '../../testing/executor-context';
import goTidyExecutor from './executor';

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: true })),
}));

describe('[executor] go-tidy', () => {
  it('should run the packaged go-tidy.sh in the project directory with the workspace root', async () => {
    const result = await goTidyExecutor({ args: ['-v'] }, executorContext());

    expect(result).toEqual({ success: true });
    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('go-tidy.sh'),
      args: ['-v'],
      cwd: '/workspace/libs/backend/go/domain',
      env: expect.objectContaining({ DMPF_WORKSPACE_ROOT: '/workspace' }),
    });
  });
});
