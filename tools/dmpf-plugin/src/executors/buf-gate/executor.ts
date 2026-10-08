import { relative } from 'node:path';
import type { ExecutorContext } from '@nx/devkit';
import { projectDirOf, runScript, scriptPath, workspaceEnv } from '../../lib/run';
import type { BufGateExecutorSchema } from './schema';

const bufGateExecutor = (
  options: BufGateExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> =>
  runScript({
    script: scriptPath('buf-gate.sh'),
    args: [
      options.gate,
      options.module ?? relative(context.root, projectDirOf(context)),
      '--project',
      options.project ?? String(context.projectName),
    ],
    cwd: context.root,
    env: workspaceEnv(context),
  });

export default bufGateExecutor;
