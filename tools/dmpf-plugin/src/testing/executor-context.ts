import type { ExecutorContext } from '@nx/devkit';

export const executorContext = (
  projectRoot = 'libs/backend/go/domain',
  root = '/workspace',
): ExecutorContext =>
  ({
    root,
    cwd: root,
    isVerbose: false,
    projectName: 'domain',
    projectsConfigurations: { version: 2, projects: { domain: { root: projectRoot } } },
    nxJsonConfiguration: {},
    projectGraph: { nodes: {}, dependencies: {} },
  }) as unknown as ExecutorContext;
