const ALLOWED_HOSTS = ['dmpf-bff', 'localhost', '127.0.0.1'];
const BASE_URL_FORMAT = /^([a-z][a-z0-9+.-]*):\/\/([^/:?#]+)(?::(\d{1,5}))?\/?$/i;

export function assertLocalTarget(baseUrl, name = 'BASE_URL') {
  const match = BASE_URL_FORMAT.exec(baseUrl);
  if (!match) {
    throw new Error(
      `guard: ${name} inválida, esperado http://<host>[:porta] (recebido "${baseUrl}")`,
    );
  }
  const [, scheme, host] = match;
  if (scheme.toLowerCase() !== 'http' || !ALLOWED_HOSTS.includes(host.toLowerCase())) {
    throw new Error(
      `guard: ${name} fora da allowlist (http em ${ALLOWED_HOSTS.join(', ')}) (recebido "${baseUrl}")`,
    );
  }
  return baseUrl.replace(/\/$/, '');
}
