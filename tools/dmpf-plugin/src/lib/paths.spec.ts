import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { packageRoot, templatesDir } from './paths';

describe('[lib] paths', () => {
  it('should resolve the root of the package that publishes templates, scripts and versions', () => {
    const manifest = JSON.parse(readFileSync(join(packageRoot(), 'package.json'), 'utf-8'));

    expect(manifest.name).toBe('@mateusmacedo/dmpf-plugin');
  });

  it('should resolve the bounded-context templates outside the compiled sources', () => {
    expect(existsSync(templatesDir('bounded-context', 'module'))).toBe(true);
  });
});
