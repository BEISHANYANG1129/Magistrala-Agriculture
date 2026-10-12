const finite = (n) => n !== null && n !== '' && Number.isFinite(Number(n))
export function mapPoints(
  rows,
  { floorPlan = false, latitudeKey = 'latitude', longitudeKey = 'longitude' } = {},
) {
  const points = [],
    parts = new Map()
  const time = (r) => {
    const n = Number(r.time ?? r.created)
    return Number.isFinite(n) ? n : Date.parse(r.created) || 0
  }
  for (const r of rows) {
    let data = r.string_value ?? r.data_value ?? r.value ?? r
    if (typeof data === 'string') {
      try {
        data = JSON.parse(data)
      } catch {
        data = null
      }
    }
    const lat =
      data?.[latitudeKey] ?? data?.lat ?? (floorPlan ? data?.y : undefined) ?? r[latitudeKey]
    const lng =
      data?.[longitudeKey] ?? data?.lng ?? (floorPlan ? data?.x : undefined) ?? r[longitudeKey]
    if (finite(lat) && finite(lng))
      points.push({
        lat: Number(lat),
        lng: Number(lng),
        publisher: r.publisher || '',
        time: time(r),
        name: r.name || r.publisher || '',
        color: r.color || r._color,
        icon: r.icon || r._icon,
        label: r._label || r.name || r.publisher || '',
      })
    else if (
      ['latitude', 'lat', 'longitude', 'lng', latitudeKey, longitudeKey].includes(r.name) &&
      finite(r.value)
    ) {
      const key = (r.publisher || '') + '|' + time(r),
        p = parts.get(key) || {
          publisher: r.publisher || '',
          time: time(r),
          name: r.publisher || '',
          color: r.color || r._color,
          icon: r.icon || r._icon,
          label: r._label || r.publisher || '',
        }
      p[['latitude', 'lat', latitudeKey].includes(r.name) ? 'lat' : 'lng'] = Number(r.value)
      parts.set(key, p)
    }
  }
  points.push(...parts.values())
  return points
    .filter(
      (p) =>
        Number.isFinite(p.lat) &&
        Number.isFinite(p.lng) &&
        (floorPlan || (Math.abs(p.lat) <= 90 && Math.abs(p.lng) <= 180)),
    )
    .sort((a, b) => a.time - b.time)
}
