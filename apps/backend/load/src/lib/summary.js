const STATUSES = ['200', '201', '204', '404', '409', '422', '429', '500', '503', '504', '0'];
const KINDS = ['read', 'write', 'poll'];
const STATUS_PROBE = 'count>=0';
const KIND_PROBE = 'max>=0';
const DURATION_UNITS_S = { s: 1, m: 60, h: 3600 };

export const SUMMARY_SUBMETRICS = Object.fromEntries([
  ...STATUSES.map((status) => [`http_reqs{status:${status}}`, [STATUS_PROBE]]),
  ...KINDS.map((kind) => [`http_req_duration{kind:${kind}}`, [KIND_PROBE]]),
]);

const isProbe = (metric, condition) =>
  metric in SUMMARY_SUBMETRICS && SUMMARY_SUBMETRICS[metric].includes(condition);

const value = (data, metric, stat) => data.metrics[metric]?.values?.[stat] ?? 0;

const formatMs = (millis) => (millis === 0 ? '—' : millis.toFixed(1));

function durationSeconds(duration) {
  const [, amount, unit] = /^(\d+)([smh])$/.exec(duration);
  return Number(amount) * DURATION_UNITS_S[unit];
}

export function breakpointRateAtAbort(shape, rate, scheduledIterations) {
  const [ramp] = shape.stages;
  const rampS = durationSeconds(ramp.duration);
  const peak = rate * ramp.multiplier;
  const elapsed = Math.min(rampS, Math.sqrt((2 * scheduledIterations * rampS) / peak));
  return (peak * elapsed) / rampS;
}

function thresholdRows(data) {
  const rows = [];
  for (const [metric, entry] of Object.entries(data.metrics)) {
    for (const [condition, result] of Object.entries(entry.thresholds ?? {})) {
      if (!isProbe(metric, condition)) {
        rows.push({ metric, condition, ok: result.ok });
      }
    }
  }
  return rows.sort((a, b) => a.metric.localeCompare(b.metric));
}

export function renderSummary(data, { params, tenants, shape, rate }) {
  const thresholds = thresholdRows(data);
  const failed = thresholds.filter((row) => !row.ok).length;
  const iterations = value(data, 'iterations', 'count');
  const dropped = value(data, 'dropped_iterations', 'count');
  const total = value(data, 'http_reqs', 'count');
  const byStatus = STATUSES.map((status) => [
    status,
    value(data, `http_reqs{status:${status}}`, 'count'),
  ]);
  const others = total - byStatus.reduce((sum, [, count]) => sum + count, 0);

  const lines = [
    `# Resultado da carga — ${params.testid}`,
    '',
    `- Perfil: \`${params.profile}\`, ${rate}, ${tenants} tenant(s)`,
    `- Tempo decorrido: ${(data.state.testRunDurationMs / 1000).toFixed(1)} s`,
    `- Resultado: ${failed === 0 ? 'todos os thresholds passaram' : `${failed} threshold(s) falharam`}`,
    `- Iterações: ${iterations} (descartadas: ${dropped})`,
    `- Recusas de admissão: ${value(data, 'admission_rejections', 'passes')}, das quais do contexto: ${value(data, 'context_admission_rejections', 'count')}`,
  ];
  if (params.profile === 'breakpoint') {
    const target = breakpointRateAtAbort(shape, params.rate, iterations + dropped);
    lines.push(`- Taxa-alvo no fim: ${target.toFixed(1)} iterações/s`);
  }
  lines.push('', '## Thresholds', '', '| Métrica | Condição | Resultado |', '| --- | --- | --- |');
  for (const row of thresholds) {
    lines.push(`| \`${row.metric}\` | \`${row.condition}\` | ${row.ok ? 'ok' : 'falhou'} |`);
  }
  lines.push('', '## Requisições por status', '', '| Status | Requisições |', '| --- | --- |');
  for (const [status, count] of byStatus) {
    if (count > 0) {
      lines.push(`| ${status === '0' ? 'erro de transporte' : status} | ${count} |`);
    }
  }
  if (others > 0) {
    lines.push(`| outros | ${others} |`);
  }
  lines.push(`| total | ${total} |`);
  lines.push('', '## Latência por tipo (ms)', '', '| Tipo | p95 | p99 |', '| --- | --- | --- |');
  for (const kind of KINDS) {
    const metric = `http_req_duration{kind:${kind}}`;
    lines.push(
      `| ${kind} | ${formatMs(value(data, metric, 'p(95)'))} | ${formatMs(value(data, metric, 'p(99)'))} |`,
    );
  }
  return `${lines.join('\n')}\n`;
}
