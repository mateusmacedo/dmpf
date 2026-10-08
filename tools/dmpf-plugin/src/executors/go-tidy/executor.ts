import type { ExecutorContext } from '@nx/devkit';
import { projectDirOf, runScript, scriptPath, workspaceEnv } from '../../lib/run';
import type { GoTidyExecutorSchema } from './schema';

const goTidyExecutor = (
  options: GoTidyExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> =>
  runScript({
    script: scriptPath('go-tidy.sh'),
    args: options.args ?? [],
    cwd: projectDirOf(context),
    env: workspaceEnv(context),
  });

export default goTidyExecutor;
