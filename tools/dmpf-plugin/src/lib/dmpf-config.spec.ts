import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createTreeWithEmptyWorkspace } from '@nx/devkit/testing';
import {
  DMPF_CONFIG_SCHEMA,
  parseDmpfConfig,
  RESERVED_MODULE_PREFIX,
  readDmpfConfig,
  readDmpfConfigFromTree,
} from './dmpf-config';

const platform = {
  schema: DMPF_CONFIG_SCHEMA,
  modulePrefix: 'github.com/mateusmacedo/dmpf',
  npmScope: '@mateusmacedo',
  appsDir: 'apps/backend',
  edge: 'bff',
  spiffeTrustDomain: 'dmpf',
  composeProfile: 'dmpf',
  composeProject: 'dmpf',
  imageRegistry: 'ghcr.io/mateusmacedo',
  bufModule: 'buf.build/mateusmacedo',
  tooling: { mode: 'local' },
};

describe('[lib] dmpf-config', () => {
  it('should parse the layout of a workspace, dropping the schema', () => {
    const { schema: _schema, ...expected } = platform;

    expect(parseDmpfConfig(platform, 'dmpf.json')).toEqual(expected);
  });

  it('should accept a workspace without a public edge and the reserved module prefix', () => {
    const config = parseDmpfConfig(
      { ...platform, edge: '', modulePrefix: RESERVED_MODULE_PREFIX, tooling: { mode: 'version' } },
      'dmpf.json',
    );

    expect(config).toMatchObject({
      edge: '',
      modulePrefix: 'example.com/change-me',
      tooling: { mode: 'version' },
    });
  });

  it.each([
    ['schema', { ...platform, schema: 'dmpf/workspace@2' }],
    ['modulePrefix', { ...platform, modulePrefix: 'Not A Path' }],
    ['npmScope', { ...platform, npmScope: 'mateusmacedo' }],
    ['appsDir', { ...platform, appsDir: '../apps' }],
    ['edge', { ...platform, edge: 'Edge_1' }],
    ['spiffeTrustDomain', { ...platform, spiffeTrustDomain: 'acme test' }],
    ['composeProfile', { ...platform, composeProfile: '' }],
    ['composeProject', { ...platform, composeProject: 'dmpf Local' }],
    ['imageRegistry', { ...platform, imageRegistry: 'GHCR.io/Acme' }],
    ['bufModule', { ...platform, bufModule: 'mateusmacedo' }],
    ['tooling.mode', { ...platform, tooling: { mode: 'remote' } }],
  ])('should refuse a dmpf.json whose %s is invalid, naming the field', (field, raw) => {
    expect(() => parseDmpfConfig(raw, 'dmpf.json')).toThrow(field);
  });

  it('should read dmpf.json from the workspace root and from the Nx tree', () => {
    const root = mkdtempSync(join(tmpdir(), 'dmpf-config-'));
    writeFileSync(join(root, 'dmpf.json'), JSON.stringify(platform));
    const tree = createTreeWithEmptyWorkspace();
    tree.write('dmpf.json', JSON.stringify(platform));

    expect(readDmpfConfig(root).appsDir).toBe('apps/backend');
    expect(readDmpfConfigFromTree(tree).edge).toBe('bff');
  });

  it('should tell how to create dmpf.json when the workspace has none', () => {
    const root = mkdtempSync(join(tmpdir(), 'dmpf-config-'));

    expect(() => readDmpfConfig(root)).toThrow('@mateusmacedo/dmpf-plugin:init');
    expect(() => readDmpfConfigFromTree(createTreeWithEmptyWorkspace())).toThrow(
      '@mateusmacedo/dmpf-plugin:init',
    );
  });
});
