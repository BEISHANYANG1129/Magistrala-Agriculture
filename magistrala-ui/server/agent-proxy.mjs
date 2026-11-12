export function createAgentProxy(target = process.env.AGRI_AGENT_URL || 'http://127.0.0.1:9026') {
  const upstreamOrigin = new URL(target).origin
  return {
    target,
    changeOrigin: true,
    timeout: 625000,
    proxyTimeout: 620000,
    rewrite: (path) => path.replace(/^\/api\/agent/, ''),
    configure(proxy) {
      proxy.on('proxyReq', (proxyReq, req) => {
        const origin = req.headers.origin
        if (origin === undefined) return
        const scheme = req.socket.encrypted ? 'https' : 'http'
        // Translate only the frontend's own origin. The loopback backend must
        // still reject foreign origins, including its own address sent here.
        proxyReq.setHeader(
          'Origin',
          origin === `${scheme}://${req.headers.host}` ? upstreamOrigin : 'null',
        )
      })
    },
  }
}
