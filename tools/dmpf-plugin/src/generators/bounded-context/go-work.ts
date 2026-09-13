export type GoWorkDocument = {
  goVersion: string;
  useEntries: readonly string[];
};

const USE_BLOCK = /^use \(\n([\s\S]*?)\n\)$/m;
const GO_DIRECTIVE = /^go[ \t]+(\S+)[ \t]*$/m;
const LEADING_BLANKS = /^([ \t]+)/;

const byCodeUnit = (left: string, right: string): number => {
  if (left < right) {
    return -1;
  }
  return left > right ? 1 : 0;
};

const useBlockOf = (content: string): RegExpExecArray => {
  const block = USE_BLOCK.exec(content);
  if (block === null) {
    throw new Error(
      'bounded-context: go.work has no "use ( ... )" block to register the modules in',
    );
  }
  return block;
};

const entryLinesOf = (block: RegExpExecArray): string[] =>
  block[1].split('\n').filter((line) => line.trim().length > 0);

export const parseGoWork = (content: string): GoWorkDocument => {
  const directive = GO_DIRECTIVE.exec(content);
  if (directive === null) {
    throw new Error('bounded-context: go.work declares no "go <version>" directive');
  }
  return {
    goVersion: directive[1],
    useEntries: entryLinesOf(useBlockOf(content)).map((line) => line.trim()),
  };
};

export const registerModules = ({
  content,
  modules,
}: {
  content: string;
  modules: readonly string[];
}): string => {
  const block = useBlockOf(content);
  const lines = entryLinesOf(block);
  const indent = LEADING_BLANKS.exec(lines[0] ?? '')?.[1] ?? '\t';
  const merged = [...lines.map((line) => line.trim()), ...modules].sort(byCodeUnit);
  const rebuilt = `use (\n${merged.map((entry) => `${indent}${entry}`).join('\n')}\n)`;
  return content.replace(block[0], () => rebuilt);
};

export const useEntryOf = (moduleDirectory: string): string => `./${moduleDirectory}`;
