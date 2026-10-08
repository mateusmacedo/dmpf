import type { ExecutorContext } from '@nx/devkit';
import { layoutEnv, runScript, scriptPath } from '../../lib/run';
import type { GateCheckExecutorSchema } from './schema';

const gateCheckExecutor = (
  _options: GateCheckExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> =>
  runScript({
    script: scriptPath('dmpf-gate-check.sh'),
    args: [],
    cwd: context.root,
    env: layoutEnv(context),
  });

export default gateCheckExecutor;
