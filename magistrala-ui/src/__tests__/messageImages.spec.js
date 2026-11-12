import { describe, expect, it } from 'vitest'
import { messageImage } from '../api/messageImages'

const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aV9sAAAAASUVORK5CYII='
describe('stored binary image messages', () => {
  it('previews Reader data_value and SenML vd without modifying the bytes', () => {
    expect(messageImage({ data_value: png })).toEqual({
      src: `data:image/png;base64,${png}`, extension: 'png', bytes: atob(png).length,
    })
    const jpeg = btoa('\xff\xd8\xff\xe0jpeg\xff\xd9')
    expect(messageImage({ vd: jpeg }).src).toBe(`data:image/jpeg;base64,${jpeg}`)
    expect(messageImage({ vd: jpeg.replace(/\+/g, '-').replace(/\//g, '_') }).extension).toBe('jpg')
  })
  it('keeps normal telemetry and text out of the image renderer', () => {
    for (const row of [{ value: 20 }, { string_value: png }, { data_value: btoa('ordinary binary') }, {}])
      expect(messageImage(row)).toBeNull()
  })
  it('rejects external URLs, executable formats, malformed or oversized data', () => {
    for (const value of ['https://example.com/image.jpg', 'data:image/png;base64,' + png, btoa('<svg onload="alert(1)"/>'), '/9j/!', 'A'.repeat(1400001)])
      expect(messageImage({ data_value: value })).toBeNull()
  })
})
