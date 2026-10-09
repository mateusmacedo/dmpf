import { join } from 'node:path';

// src/lib e dist/lib ficam dois níveis abaixo da raiz do pacote, onde templates/,
// scripts/ e versions.json são publicados fora do código compilado.
export const packageRoot = (): string => join(__dirname, '..', '..');

export const templatesDir = (...segments: string[]): string =>
  join(packageRoot(), 'templates', ...segments);
