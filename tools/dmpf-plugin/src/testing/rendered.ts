import { createHash } from 'node:crypto';
import type { Tree } from '@nx/devkit';
import { readJson, writeJson } from '@nx/devkit';
import { createTreeWithEmptyWorkspace } from '@nx/devkit/testing';

export const consumerTree = (): Tree => {
  const tree = createTreeWithEmptyWorkspace();
  tree.write('package.json', JSON.stringify({ name: '@acme/shop', devDependencies: {} }));
  return tree;
};

export const renderedByAnOlderVersion = (tree: Tree, path: string, content: string): void => {
  tree.write(path, content);
  const record = readJson(tree, 'dmpf.rendered.json');
  record.files[path] = createHash('sha256').update(content).digest('hex');
  writeJson(tree, 'dmpf.rendered.json', record);
};

export const snapshot = (tree: Tree, paths: readonly string[]): Record<string, string | null> =>
  Object.fromEntries(paths.map((path) => [path, tree.read(path, 'utf-8')]));
