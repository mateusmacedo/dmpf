import { execFileSync } from 'node:child_process';
import { type ExecutorContext, logger } from '@nx/devkit';
import { readDmpfConfig } from '../../lib/dmpf-config';
import { layoutEnv, runScript, scriptPath } from '../../lib/run';
import type { ContextCheckExecutorSchema } from './schema';

export const KERNEL_POSTGRES_MODULE = 'github.com/mateusmacedo/dmpf/libs/backend/go/postgres';
const LOCAL_KERNEL_DDL = 'libs/backend/go/postgres';

const kernelDdlOf = (root: string, mode: 'local' | 'version'): string =>
  mode === 'local'
    ? LOCAL_KERNEL_DDL
    : execFileSync('go', ['list', '-m', '-f', '{{.Dir}}', KERNEL_POSTGRES_MODULE], {
        cwd: root,
        encoding: 'utf-8',
      }).trim();

const contextCheckExecutor = async (
  options: ContextCheckExecutorSchema,
  context: ExecutorContext,
): Promise<{ success: boolean }> => {
  const phase = options.phase ?? 'structural';
  const { tooling } = readDmpfConfig(context.root);
  if (phase === 'self-test' && tooling.mode !== 'local') {
    logger.error(
      'context-check: the self-test builds its fixture from the platform kernel and runs only in tooling.mode local',
    );
    return { success: false };
  }
  return runScript({
    script: scriptPath('dmpf-context-check.sh'),
    args: ['--phase', phase, ...(options.context ? ['--context', options.context] : [])],
    cwd: context.root,
    env: { ...layoutEnv(context), DMPF_KERNEL_DDL: kernelDdlOf(context.root, tooling.mode) },
  });
};

export default contextCheckExecutor;
