// Only inline raster bytes from stored messages are eligible for preview.
// Do not interpret strings, external URLs or SVG/HTML messages as images.
export function messageImage(row) {
  const value = row?.data_value ?? row?.vd
  if (typeof value !== 'string' || !value || value.length > 1400000) return null
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/')
  if (!/^[A-Za-z0-9+/]+={0,2}$/.test(base64)) return null
  let bytes
  try {
    bytes = atob(base64)
  } catch {
    return null
  }
  const jpeg = bytes.startsWith('\xff\xd8\xff')
  const png = bytes.startsWith('\x89PNG\r\n\x1a\n')
  if (!jpeg && !png) return null
  const mime = jpeg ? 'image/jpeg' : 'image/png'
  return { src: `data:${mime};base64,${base64}`, extension: jpeg ? 'jpg' : 'png', bytes: bytes.length }
}
