// The nonce the server stamps into index.html per response (vite.config.ts places the
// placeholder). Anything that injects a style element at runtime must carry it.
export const cspNonce =
  document.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]')?.nonce ?? ''
