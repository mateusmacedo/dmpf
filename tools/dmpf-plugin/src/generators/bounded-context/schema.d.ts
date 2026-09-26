export type Block = 'domain' | 'port' | 'application' | 'provider' | 'app';

export type BoundedContextGeneratorSchema = {
  name: string;
  boundedContext: string;
  blocks?: string[];
  directory?: string;
  serviceName?: string;
};
