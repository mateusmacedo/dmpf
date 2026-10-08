import { logger, type Tree } from '@nx/devkit';
import {
  applyTemplates,
  mergeNxJson,
  planTemplates,
  type TemplatePlan,
} from '../../generators/init/generator';
import { DMPF_CONFIG_FILE, readDmpfConfigFromTree } from '../../lib/dmpf-config';
import { readVersions } from '../../lib/versions';

const CI_CALLER = '.github/workflows/dmpf-ci.yml';
const PLUGIN_OWNED_LINES = [
  /^[ \t]*uses: mateusmacedo\/dmpf\/\.github\/workflows\/dmpf-go-ci\.yml@\S+$/m,
  /^[ \t]*plugin-version: "[^"]*"$/m,
];

// An edited CI caller keeps its edits, but the workflow ref and the plugin version
// must follow the installed plugin, or the reusable workflow rejects the run.
const followPluginInCaller = (tree: Tree, plan: TemplatePlan): void => {
  const current = tree.read(CI_CALLER, 'utf-8');
  const rendered = plan.rendered.get(CI_CALLER)?.content;
  if (!plan.edited.includes(CI_CALLER) || current === null || rendered === undefined) {
    return;
  }
  let next = current;
  for (const line of PLUGIN_OWNED_LINES) {
    const wanted = line.exec(rendered)?.[0];
    if (wanted !== undefined) {
      next = next.replace(line, () => wanted);
    }
  }
  if (next !== current) {
    tree.write(CI_CALLER, next);
    logger.warn(
      `dmpf-plugin: ${CI_CALLER} kept its edits, but its workflow ref and plugin version now follow the installed plugin.`,
    );
  }
};

// nextSteps are run as commands after the upgrade, so the edited files go to the
// log instead.
const syncInit = (tree: Tree): void => {
  if (!tree.exists(DMPF_CONFIG_FILE)) {
    return;
  }
  const versions = readVersions();
  const plan = planTemplates(tree, readDmpfConfigFromTree(tree), versions);
  applyTemplates(tree, plan);
  followPluginInCaller(tree, plan);
  mergeNxJson(tree, versions);
  if (plan.edited.length > 0) {
    logger.warn(
      `dmpf-plugin: kept as edited, not updated: ${plan.edited.join(', ')}. Run "pnpm nx g @mateusmacedo/dmpf-plugin:init --force" to overwrite them.`,
    );
  }
};

export default syncInit;
