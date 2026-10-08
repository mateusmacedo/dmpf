import type { ExecutorContext } from '@nx/devkit';
import { projectDirOf, runScript, scriptPath, workspaceEnv } from '../../lib/run';
import type { TestEnvExecutorSchema } from './schema';

const testEnvExecutor = (
  options: TestEnvExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> =>
  runScript({
    script: scriptPath('test-env.sh'),
    args: ['bash', '-c', options.command],
    cwd: projectDirOf(context),
    env: workspaceEnv(context),
  });

export default testEnvExecutor;
