export type ContextIdentifiers = {
  goIdent: string;
};

export const IDENTIFIER_PATTERN = /^[a-z][a-z0-9-]*$/;

export const isIdentifier = (value: string): boolean => IDENTIFIER_PATTERN.test(value);

export const identifiersOf = (name: string): ContextIdentifiers => ({
  goIdent: name.replaceAll('-', ''),
});
