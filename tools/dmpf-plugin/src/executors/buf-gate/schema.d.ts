export type BufGateExecutorSchema = {
  gate: 'warmup' | 'lint' | 'pins' | 'generate-check' | 'breaking';
  module?: string;
  project?: string;
};
