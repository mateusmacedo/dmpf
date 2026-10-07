import { readJson } from '@nx/devkit';
import { createTreeWithEmptyWorkspace } from '@nx/devkit/testing';
import { readVersions } from '../../lib/versions';
import updateVersions from './update-versions';

const KERNEL = 'github.com/mateusmacedo/dmpf/libs/backend/go';

const goMod = (kernel: string): string => `module example.com/consumer/apps/backend/shop

go 1.26

require (
\t${KERNEL}/domain ${kernel}
\t${KERNEL}/postgres ${kernel} // indirect
\tgithub.com/jackc/pgx/v5 v5.7.0
)

require ${KERNEL}/ports ${kernel}

replace (
\t${KERNEL}/app v0.1.0 => ../app
)
`;

describe('[migration] update-versions', () => {
  it('should move the kernel requires of every go.mod and leave other modules and replaces alone', async () => {
    const tree = createTreeWithEmptyWorkspace();
    const { kernel } = readVersions();
    tree.write('apps/backend/shop/go.mod', goMod('v0.9.0'));

    const nextSteps = await updateVersions(tree);

    expect(tree.read('apps/backend/shop/go.mod', 'utf-8')).toBe(goMod(kernel));
    expect(nextSteps).toContain('pnpm nx run-many -t tidy');
  });

  it('should move the pins of govulncheck, golangci-lint, @nx-go/nx-go and the go directive', async () => {
    const tree = createTreeWithEmptyWorkspace();
    const versions = readVersions();
    tree.write(
      'apps/backend/shop/project.json',
      JSON.stringify({ command: 'go run golang.org/x/vuln/cmd/govulncheck@v1.0.0 ./...' }),
    );
    tree.write(
      'nx.json',
      JSON.stringify({
        args: ['run', 'github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.0'],
      }),
    );
    tree.write('package.json', JSON.stringify({ devDependencies: { '@nx-go/nx-go': '3.0.0' } }));
    tree.write('go.work', 'go 1.25.0\n\nuse ./apps/backend/shop\n');

    const nextSteps = await updateVersions(tree);

    expect(tree.read('apps/backend/shop/project.json', 'utf-8')).toContain(
      `govulncheck@${versions.govulncheck} `,
    );
    expect(tree.read('nx.json', 'utf-8')).toContain(`golangci-lint@${versions.golangciLint}"`);
    expect(readJson(tree, 'package.json').devDependencies['@nx-go/nx-go']).toBe(versions.nxGo);
    expect(tree.read('go.work', 'utf-8')).toBe(
      `go ${versions.go.directive}\n\nuse ./apps/backend/shop\n`,
    );
    expect(nextSteps).toEqual(['pnpm install']);
  });

  it('should ask for nothing when the workspace is already on this version', async () => {
    const tree = createTreeWithEmptyWorkspace();
    const { kernel, go } = readVersions();
    tree.write('apps/backend/shop/go.mod', goMod(kernel));
    tree.write('go.work', `go ${go.directive}\n`);

    expect(await updateVersions(tree)).toEqual([]);
  });
});
