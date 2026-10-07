import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { ToolingMode } from '../lib/dmpf-config';

export const workspaceWithDmpfJson = (mode: ToolingMode, appsDir = 'apps/backend'): string => {
  const root = mkdtempSync(join(tmpdir(), 'dmpf-workspace-'));
  writeFileSync(
    join(root, 'dmpf.json'),
    JSON.stringify({
      schema: 'dmpf/workspace@1',
      modulePrefix: 'example.com/consumer',
      npmScope: '@consumer',
      appsDir,
      edge: '',
      spiffeTrustDomain: 'consumer',
      composeProfile: 'consumer',
      imageRegistry: 'ghcr.io/consumer',
      bufModule: 'buf.build/consumer',
      tooling: { mode },
    }),
  );
  return root;
};
