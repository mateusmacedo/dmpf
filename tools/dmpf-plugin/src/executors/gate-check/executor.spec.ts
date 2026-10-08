import { runScript, scriptPath } from '../../lib/run';
import { executorContext } from '../../testing/executor-context';
import { workspaceWithDmpfJson } from '../../testing/workspace';
import gateCheckExecutor from './executor';

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: true })),
}));

describe('[executor] gate-check', () => {
  it('should run the dependency gate from the workspace root with the apps directory of dmpf.json', async () => {
    const root = workspaceWithDmpfJson('local', 'services');

    const result = await gateCheckExecutor({}, executorContext('libs/backend/go/domain', root));

    expect(result).toEqual({ success: true });
    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('dmpf-gate-check.sh'),
      args: [],
      cwd: root,
      env: expect.objectContaining({ DMPF_WORKSPACE_ROOT: root, DMPF_APPS_DIR: 'services' }),
    });
  });
});
