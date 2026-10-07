import { basename } from 'node:path';
import { readJson, type Tree, visitNotIgnoredFiles } from '@nx/devkit';
import { pinGoWork } from '../../generators/init/generator';
import { readVersions } from '../../lib/versions';

const NX_GO = '@nx-go/nx-go';
const KERNEL_REQUIRE =
  /^(\s*(?:require\s+)?github\.com\/mateusmacedo\/dmpf\/libs\/backend\/go\/[a-z0-9_-]+\s+)v\S+(?![^\n]*=>)/gm;
const GOVULNCHECK = /(golang\.org\/x\/vuln\/cmd\/govulncheck@)v[^\s"]+/g;
const GOLANGCI_LINT = /(github\.com\/golangci\/golangci-lint\/v2\/cmd\/golangci-lint@)v[^\s"]+/g;

const rewrite = (tree: Tree, path: string, pattern: RegExp, replacement: string): boolean => {
  const current = tree.read(path, 'utf-8') ?? '';
  const next = current.replace(pattern, replacement);
  if (next === current) {
    return false;
  }
  tree.write(path, next);
  return true;
};

const bumpNxGo = (tree: Tree, version: string): boolean => {
  const manifest = readJson(tree, 'package.json');
  const field = manifest.devDependencies?.[NX_GO] ? 'devDependencies' : 'dependencies';
  if (!manifest[field]?.[NX_GO] || manifest[field][NX_GO] === version) {
    return false;
  }
  manifest[field][NX_GO] = version;
  tree.write('package.json', `${JSON.stringify(manifest, null, 2)}\n`);
  return true;
};

// The go.sum of a module only follows its new require after a tidy, which runs
// go and so cannot happen inside a migration.
const updateVersions = async (tree: Tree): Promise<string[]> => {
  const versions = readVersions();
  const moved: string[] = [];
  visitNotIgnoredFiles(tree, '', (path) => {
    if (
      basename(path) === 'go.mod' &&
      rewrite(tree, path, KERNEL_REQUIRE, `$1${versions.kernel}`)
    ) {
      moved.push(path);
    } else if (basename(path) === 'project.json') {
      rewrite(tree, path, GOVULNCHECK, `$1${versions.govulncheck}`);
    }
  });
  if (tree.exists('nx.json')) {
    rewrite(tree, 'nx.json', GOLANGCI_LINT, `$1${versions.golangciLint}`);
  }
  pinGoWork(tree, versions);
  const install = tree.exists('package.json') && bumpNxGo(tree, versions.nxGo);
  return [
    ...(install ? ['pnpm install'] : []),
    ...(moved.length > 0 ? ['pnpm nx run-many -t tidy'] : []),
  ];
};

export default updateVersions;
