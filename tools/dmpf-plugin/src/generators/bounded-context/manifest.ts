import type { ExternalDependency } from './blocks';

const LINE_WIDTH = 100;
const INDENT = '  ';

// Biome collapses a JSON array onto one line when it fits lineWidth (biome.json: 100),
// expands it otherwise and never re-collapses a multi-line object; reproducing that
// decision keeps `biome ci` a no-op on the generated manifest for any context name.
const stringArray = ({
  level,
  key,
  items,
  trailing,
}: {
  level: number;
  key: string;
  items: readonly string[];
  trailing: string;
}): string => {
  const pad = INDENT.repeat(level);
  const head = `${pad}"${key}": `;
  if (items.length === 0) {
    return `${head}[]${trailing}`;
  }
  const quoted = items.map((item) => JSON.stringify(item));
  const inline = `${head}[${quoted.join(', ')}]${trailing}`;
  if (inline.length <= LINE_WIDTH) {
    return inline;
  }
  const body = quoted.map((item) => `${pad}${INDENT}${item}`).join(',\n');
  return `${head}[\n${body}\n${pad}]${trailing}`;
};

export const includeFragment = ({
  level,
  packages,
}: {
  level: number;
  packages: readonly string[];
}): string => stringArray({ level, key: 'include', items: packages, trailing: '' });

export const externalFragment = ({
  level,
  external,
}: {
  level: number;
  external: readonly ExternalDependency[];
}): string => {
  const pad = INDENT.repeat(level);
  if (external.length === 0) {
    return `${pad}"external": [],`;
  }
  const entries = external.map((dependency) =>
    [
      `${pad}${INDENT}{`,
      `${pad}${INDENT.repeat(2)}"package": ${JSON.stringify(dependency.package)},`,
      `${pad}${INDENT.repeat(2)}"versions": ${JSON.stringify(dependency.versions)},`,
      stringArray({
        level: level + 2,
        key: 'entrypoints',
        items: dependency.entrypoints,
        trailing: ',',
      }),
      `${pad}${INDENT.repeat(2)}"capability": ${JSON.stringify(dependency.capability)}`,
      `${pad}${INDENT}}`,
    ].join('\n'),
  );
  return [`${pad}"external": [`, entries.join(',\n'), `${pad}],`].join('\n');
};
