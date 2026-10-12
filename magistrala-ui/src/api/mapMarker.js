import { createElement, icons } from 'lucide'

const normalize = (name) =>
  String(name || '')
    .replace(/[^a-z0-9]/gi, '')
    .toLowerCase()
const byName = new Map(Object.entries(icons).map(([name, node]) => [normalize(name), node]))

export function mapMarker(name, color) {
  const wrapper = document.createElement('span')
  wrapper.className = 'map-marker-icon'
  wrapper.style.color = color || '#17725f'
  const svg = createElement(byName.get(normalize(name)) || icons.MapPin, {
    width: 28,
    height: 28,
    'stroke-width': 2,
    'aria-hidden': 'true',
  })
  wrapper.appendChild(svg)
  return wrapper
}
