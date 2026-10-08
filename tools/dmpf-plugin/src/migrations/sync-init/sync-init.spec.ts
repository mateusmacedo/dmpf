import { logger } from '@nx/devkit';
import { initGenerator } from '../../generators/init/generator';
import { consumerTree, renderedByAnOlderVersion } from '../../testing/rendered';
import syncInit from './sync-init';

jest.mock('../../lib/versions', () => {
  const actual = jest.requireActual<typeof import('../../lib/versions')>('../../lib/versions');
  return {
    ...actual,
    readVersions: jest.fn(() => ({ ...actual.readVersions(), workflowRef: 'f'.repeat(40) })),
  };
});

const OPTIONS = { modulePrefix: 'github.com/acme/shop' };

const initialized = async () => {
  const tree = consumerTree();
  await initGenerator(tree, OPTIONS);
  return tree;
};

describe('[migration] sync-init', () => {
  beforeEach(() => jest.spyOn(logger, 'warn').mockImplementation(() => undefined));
  afterEach(() => jest.restoreAllMocks());

  it('should rewrite what an older version rendered and nobody edited', async () => {
    const tree = await initialized();
    const current = tree.read('.golangci.yml', 'utf-8');
    renderedByAnOlderVersion(tree, '.golangci.yml', 'version: "2"\n');

    await syncInit(tree);

    expect(tree.read('.golangci.yml', 'utf-8')).toBe(current);
    expect(logger.warn).not.toHaveBeenCalled();
  });

  it('should keep an edited file, warn about it and leave it marked as edited for init', async () => {
    const tree = await initialized();
    tree.write('.golangci.yml', 'edited: true\n');

    await syncInit(tree);

    expect(tree.read('.golangci.yml', 'utf-8')).toBe('edited: true\n');
    expect(logger.warn).toHaveBeenCalledWith(expect.stringContaining('.golangci.yml'));
    await expect(initGenerator(tree, OPTIONS)).rejects.toThrow('.golangci.yml');
  });

  it('should move the workflow ref and the plugin version of an edited CI caller, keeping the edits', async () => {
    const tree = await initialized();
    const caller = tree.read('.github/workflows/dmpf-ci.yml', 'utf-8') as string;
    const edited = `${caller.replace(/@\S+$/m, '@old').replace(/plugin-version: "[^"]*"/, 'plugin-version: "0.0.1"')}# edição\n`;
    tree.write('.github/workflows/dmpf-ci.yml', edited);

    await syncInit(tree);

    expect(tree.read('.github/workflows/dmpf-ci.yml', 'utf-8')).toBe(`${caller}# edição\n`);
  });

  it('should leave a workspace that never ran init untouched', async () => {
    const tree = consumerTree();

    await syncInit(tree);

    expect(tree.exists('dmpf.json')).toBe(false);
    expect(tree.exists('dmpf.rendered.json')).toBe(false);
  });
});
