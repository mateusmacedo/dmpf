import { execFileSync } from 'node:child_process';
import { type ExecutorContext, logger } from '@nx/devkit';
import { readDmpfConfig } from '../../lib/dmpf-config';
import { layoutEnv, runScript, scriptPath } from '../../lib/run';
import type { ContextCheckExecutorSchema } from './schema';

export const KERNEL_POSTGRES_MODULE = 'github.com/mateusmacedo/dmpf/libs/backend/go/postgres';
const LOCAL_KERNEL_DDL = 'libs/backend/go/postgres';
const NO_KERNEL_DDL = 'none';

const inBuildList = (root: string): boolean => {
  try {
    execFileSync('go', ['list', '-m', KERNEL_POSTGRES_MODULE], { cwd: root, stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
};

// Before the first context with a provider the kernel postgres is not in the build
// list; with a cold module cache `go list -m` leaves Dir empty, whereas `go mod download` does not.
const kernelDdlOf = (root: string, mode: 'local' | 'version'): string => {
  if (mode === 'local') {
    return LOCAL_KERNEL_DDL;
  }
  if (!inBuildList(root)) {
    return NO_KERNEL_DDL;
  }
  return JSON.parse(
    execFileSync('go', ['mod', 'download', '-json', KERNEL_POSTGRES_MODULE], {
      cwd: root,
      encoding: 'utf-8',
    }),
  ).Dir;
};

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
  let kernelDdl: string;
  try {
    kernelDdl = kernelDdlOf(context.root, tooling.mode);
  } catch (error) {
    logger.error(
      `context-check: the kernel DDL could not be resolved: ${(error as Error).message}`,
    );
    return { success: false };
  }
  return runScript({
    script: scriptPath('dmpf-context-check.sh'),
    args: ['--phase', phase, ...(options.context ? ['--context', options.context] : [])],
    cwd: context.root,
    env: { ...layoutEnv(context), DMPF_KERNEL_DDL: kernelDdl },
  });
};

export default contextCheckExecutor;
