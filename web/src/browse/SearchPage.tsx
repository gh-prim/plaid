import { useSearchParams } from 'react-router'
import { SearchBox } from './SearchBox'

/** Search on its own page: the phone's Search tab (F008-R25). */
export function SearchPage() {
  const [params] = useSearchParams()
  return (
    <div className="br-grid-page">
      <SearchBox page initial={params.get('q') ?? ''} />
    </div>
  )
}
