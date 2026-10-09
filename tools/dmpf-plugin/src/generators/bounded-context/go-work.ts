export type GoWorkDocument = {
  goVersion: string;
  useEntries: readonly string[];
};

const USE_BLOCK = /^use \(\n([\s\S]*?)\n\)$/m;
const GO_DIRECTIVE = /^go[ \t]+(\S+)[ \t]*$/m;
const LEADING_BLANKS = /^([ \t]+)/;

export const byCodeUnit = (left: string, right: string): number => {
  if (left < right) {
    return -1;
  }
  return left > right ? 1 : 0;
};

const withSingleTrailingNewline = (text: string): string => {
  let end = text.length;
  while (end > 0 && text[end - 1] === '\n') {
    end -= 1;
  }
  return `${text.slice(0, end)}\n`;
};

const entryLinesOf = (block: RegExpExecArray | null): string[] =>
  (block?.[1] ?? '').split('\n').filter((line) => line.trim().length > 0);

export const parseGoWork = (content: string): GoWorkDocument => {
  const directive = GO_DIRECTIVE.exec(content);
  if (directive === null) {
    throw new Error('bounded-context: go.work declares no "go <version>" directive');
  }
  return {
    goVersion: directive[1],
    useEntries: entryLinesOf(USE_BLOCK.exec(content)).map((line) => line.trim()),
  };
};

export const registerModules = ({
  content,
  modules,
}: {
  content: string;
  modules: readonly string[];
}): string => {
  const block = USE_BLOCK.exec(content);
  const lines = entryLinesOf(block);
  const indent = LEADING_BLANKS.exec(lines[0] ?? '')?.[1] ?? '\t';
  const merged = [...lines.map((line) => line.trim()), ...modules].sort(byCodeUnit);
  const rebuilt = `use (\n${merged.map((entry) => `${indent}${entry}`).join('\n')}\n)`;
  return block === null
    ? `${withSingleTrailingNewline(content)}\n${rebuilt}\n`
    : content.replace(block[0], () => rebuilt);
};

export const useEntryOf = (moduleDirectory: string): string => `./${moduleDirectory}`;
