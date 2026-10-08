import { spawn } from 'node:child_process';
import { join } from 'node:path';
import { type ExecutorContext, logger } from '@nx/devkit';
import { readDmpfConfig } from './dmpf-config';
import { packageRoot } from './paths';
import { readVersions } from './versions';

export type ScriptRun = {
  script: string;
  args?: readonly string[];
  cwd: string;
  env?: Record<string, string>;
};

export const scriptPath = (name: string): string => join(packageRoot(), 'scripts', name);

export const runScript = ({
  script,
  args = [],
  cwd,
  env = {},
}: ScriptRun): Promise<{ success: boolean }> =>
  new Promise((resolve) => {
    const child = spawn('bash', [script, ...args], {
      cwd,
      stdio: 'inherit',
      env: { ...process.env, ...env },
    });
    child.on('error', (error) => {
      logger.error(`dmpf-plugin: ${script} did not start: ${error.message}`);
      resolve({ success: false });
    });
    child.on('close', (code) => resolve({ success: code === 0 }));
  });

export const projectDirOf = (context: ExecutorContext): string => {
  const project = context.projectName
    ? context.projectsConfigurations?.projects[context.projectName]
    : undefined;
  if (!project) {
    throw new Error('dmpf-plugin: this executor runs only as a target of a project');
  }
  return join(context.root, project.root);
};

export const workspaceEnv = (context: ExecutorContext): Record<string, string> => ({
  DMPF_WORKSPACE_ROOT: context.root,
  DMPF_BUF_VERSION: readVersions().buf,
});

export const layoutEnv = (context: ExecutorContext): Record<string, string> => ({
  ...workspaceEnv(context),
  DMPF_APPS_DIR: readDmpfConfig(context.root).appsDir,
});
