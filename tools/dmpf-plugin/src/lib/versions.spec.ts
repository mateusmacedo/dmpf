import { parseVersions, readVersions, VERSIONS_SCHEMA } from './versions';

const valid = {
  schema: VERSIONS_SCHEMA,
  kernel: 'v1.0.0-rc.1',
  conformance: 'v1.0.0-rc.1',
  go: { directive: '1.26.6', image: 'golang:1.26.6-alpine' },
  buf: 'v1.72.0',
  protocGenGo: 'v1.36.12',
  golangciLint: 'v2.13.2',
  govulncheck: 'v1.7.0',
  workflowRef: '',
};

describe('[lib] versions', () => {
  it('should read the versions.json published at the package root', () => {
    const versions = readVersions();

    expect(versions.kernel).toMatch(/^v\d+\.\d+\.\d+/);
    expect(versions.go.directive).toMatch(/^\d+\.\d+/);
  });

  it('should accept a SHA or an empty workflowRef, which the release fills in', () => {
    expect(parseVersions(valid, 'versions.json').workflowRef).toBe('');
    const sha = 'a'.repeat(40);

    expect(parseVersions({ ...valid, workflowRef: sha }, 'versions.json').workflowRef).toBe(sha);
  });

  it.each([
    ['schema', { ...valid, schema: 'dmpf/versions@2' }],
    ['kernel', { ...valid, kernel: '1.0.0' }],
    ['conformance', { ...valid, conformance: undefined }],
    ['go.directive', { ...valid, go: { ...valid.go, directive: 'latest' } }],
    ['go.image', { ...valid, go: { ...valid.go, image: '' } }],
    ['buf', { ...valid, buf: 'v1.72' }],
    ['workflowRef', { ...valid, workflowRef: 'main' }],
  ])('should refuse a manifest whose %s is malformed, naming the field', (field, manifest) => {
    expect(() => parseVersions(manifest, 'versions.json')).toThrow(field);
  });
});
