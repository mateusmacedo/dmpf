import { runScript, scriptPath } from '../../lib/run';
import { executorContext } from '../../testing/executor-context';
import bufGateExecutor from './executor';

jest.mock('../../lib/run', () => ({
  ...jest.requireActual<typeof import('../../lib/run')>('../../lib/run'),
  runScript: jest.fn(async () => ({ success: true })),
}));

beforeEach(() => jest.mocked(runScript).mockClear());

describe('[executor] buf-gate', () => {
  it.each([
    'warmup',
    'lint',
    'pins',
    'generate-check',
    'breaking',
  ] as const)('should run the %s gate on the module of the project, from the workspace root', async (gate) => {
    await bufGateExecutor({ gate }, executorContext('apps/backend/bookings/contract'));

    expect(runScript).toHaveBeenCalledWith({
      script: scriptPath('buf-gate.sh'),
      args: [gate, 'apps/backend/bookings/contract', '--project', 'domain'],
      cwd: '/workspace',
      env: expect.objectContaining({ DMPF_WORKSPACE_ROOT: '/workspace' }),
    });
  });

  it('should take the module and the project from the options when given', async () => {
    await bufGateExecutor(
      { gate: 'lint', module: 'libs/backend/go/contracts', project: 'contracts' },
      executorContext(),
    );

    expect(jest.mocked(runScript).mock.calls[0][0].args).toEqual([
      'lint',
      'libs/backend/go/contracts',
      '--project',
      'contracts',
    ]);
  });
});
