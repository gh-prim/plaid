import { Navigate, Route, Routes } from 'react-router'
import { AdminLayout } from './AdminLayout'
import { AdminSettings } from './AdminSettings'
import { Dashboard } from './Dashboard'
import { Devices } from './Devices'
import { Libraries, LibraryPage } from './Libraries'
import { People, Person } from './People'
import { Playing } from './Playing'
import { RemoteAccess } from './RemoteAccess'
import { Review, Unrecognized } from './Review'
import { Metadata, Problems, Transcoding } from './Settings'

/**
 * The admin area, mounted at /admin/* and loaded on demand: the wizard
 * and members never fetch it (F007 R17, NFR bundle).
 */
export default function AdminApp() {
  return (
    <Routes>
      <Route element={<AdminLayout />}>
        <Route index element={<Dashboard />} />
        <Route path="settings" element={<AdminSettings />} />
        <Route path="playing" element={<Playing />} />
        <Route path="libraries" element={<Libraries />} />
        <Route path="libraries/:id" element={<LibraryPage />} />
        <Route path="libraries/:id/unrecognized" element={<Unrecognized />} />
        <Route path="libraries/:id/review" element={<Review />} />
        <Route path="people" element={<People />} />
        <Route path="people/:id" element={<Person />} />
        <Route path="devices" element={<Devices />} />
        <Route path="transcoding" element={<Transcoding />} />
        <Route path="metadata" element={<Metadata />} />
        <Route path="problems" element={<Problems />} />
        <Route path="remote-access" element={<RemoteAccess />} />
        <Route path="*" element={<Navigate to="/admin" replace />} />
      </Route>
    </Routes>
  )
}
