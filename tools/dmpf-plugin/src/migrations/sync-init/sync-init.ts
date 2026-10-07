import { logger, type Tree } from '@nx/devkit';
import { applyTemplates, mergeNxJson, planTemplates } from '../../generators/init/generator';
import { DMPF_CONFIG_FILE, readDmpfConfigFromTree } from '../../lib/dmpf-config';
import { readVersions } from '../../lib/versions';

// nextSteps are run as commands after the upgrade, so the edited files go to the
// log instead.
const syncInit = async (tree: Tree): Promise<void> => {
  if (!tree.exists(DMPF_CONFIG_FILE)) {
    return;
  }
  const versions = readVersions();
  const plan = planTemplates(tree, readDmpfConfigFromTree(tree), versions);
  applyTemplates(tree, plan);
  mergeNxJson(tree, versions);
  if (plan.edited.length > 0) {
    logger.warn(
      `dmpf-plugin: kept as edited, not updated: ${plan.edited.join(', ')}. Run "pnpm nx g @mateusmacedo/dmpf-plugin:init --force" to overwrite them.`,
    );
  }
};

export default syncInit;
