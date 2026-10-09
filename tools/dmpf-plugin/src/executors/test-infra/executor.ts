import type { ExecutorContext } from '@nx/devkit';
import { runScript, scriptPath, workspaceEnv } from '../../lib/run';
import type { TestInfraExecutorSchema } from './schema';

const testInfraExecutor = (
  options: TestInfraExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> =>
  runScript({
    script: scriptPath('test-infra.sh'),
    args: [options.action],
    cwd: context.root,
    env: workspaceEnv(context),
  });

export default testInfraExecutor;
