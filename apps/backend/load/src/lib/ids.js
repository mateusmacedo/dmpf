export function idsFor(testid, vu, iteration) {
  const suffix = `${testid}-${vu}-${iteration}`;
  return {
    correlation: `load-${suffix}`,
    order: `o-${suffix}`,
    booking: `b-${suffix}`,
    resource: `r-${suffix}`,
  };
}

export function setupOrderId(testid, tenantIndex, sequence) {
  return `o-${testid}-setup-${tenantIndex}-${sequence}`;
}
