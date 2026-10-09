import type { ExecutorContext } from '@nx/devkit';
import { projectDirOf, runScript, scriptPath, workspaceEnv } from '../../lib/run';
import type { BufExecutorSchema } from './schema';

const bufExecutor = (
  options: BufExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> =>
  runScript({
    script: scriptPath('buf.sh'),
    args: options.args ?? [],
    cwd: projectDirOf(context),
    env: workspaceEnv(context),
  });

export default bufExecutor;
