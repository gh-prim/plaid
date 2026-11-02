import { geoEqualEarth, geoPath } from 'd3-geo'
import { useMemo } from 'react'
import { feature } from 'topojson-client'
import type { GeometryCollection, Topology } from 'topojson-specification'
import world from 'world-atlas/countries-110m.json'

const width = 960
const height = 470

export type MapPoint = { key: string; lat: number; lon: number; weight: number; label: string }

/**
 * The world's countries drawn here (Natural Earth through world-atlas, no
 * tile server), with a circle per place sized by its weight.
 */
export function WorldMap({ points, label }: { points: MapPoint[]; label: string }) {
  const { land, project } = useMemo(() => {
    const topo = world as unknown as Topology<{ countries: GeometryCollection }>
    const countries = feature(topo, topo.objects.countries)
    const projection = geoEqualEarth().fitSize([width, height], countries)
    return {
      land: geoPath(projection)(countries) ?? '',
      project: (lon: number, lat: number) => projection([lon, lat]),
    }
  }, [])
  const most = Math.max(1, ...points.map((p) => p.weight))
  return (
    <svg className="ax-map" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={label}>
      <path className="ax-map__land" d={land} />
      {points.map((p) => {
        const xy = project(p.lon, p.lat)
        if (!xy) return null
        const r = 4 + 14 * Math.sqrt(p.weight / most)
        return (
          <g key={p.key} className="ax-map__dot">
            <circle cx={xy[0]} cy={xy[1]} r={r} />
            <title>{p.label}</title>
          </g>
        )
      })}
    </svg>
  )
}
