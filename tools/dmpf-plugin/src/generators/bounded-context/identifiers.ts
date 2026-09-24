export const IDENTIFIER_PATTERN = /^[a-z][a-z0-9-]*$/;

export const isIdentifier = (value: string): boolean => IDENTIFIER_PATTERN.test(value);
