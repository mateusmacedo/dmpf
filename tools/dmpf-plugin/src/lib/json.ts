import { parseJson } from '@nx/devkit';

export const parseJsonOf = <T extends object = Record<string, unknown>>(
  content: string,
  source: string,
): T => {
  try {
    return parseJson<T>(content);
  } catch (error) {
    throw new Error(`${source}: invalid JSON (${(error as Error).message})`);
  }
};
