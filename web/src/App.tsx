import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router'
import { AuthProvider } from './auth/AuthProvider'
import { RequireStep } from './auth/RequireAuth'
import { CollectionPage } from './browse/CollectionPage'
import { Collections } from './browse/Collections'
import { HomePage } from './browse/HomePage'
import { ItemPage } from './browse/ItemPage'
import { PersonPage } from './browse/PersonPage'
import { SearchPage } from './browse/SearchPage'
import { LibraryGrid } from './browse/LibraryGrid'
import { Shell } from './browse/Shell'
import { ChangePassword } from './pages/ChangePassword'
import { Pair } from './pages/Pair'
import { ProfilePicker } from './pages/ProfilePicker'
import { SignIn } from './pages/SignIn'
import { Account } from './onboarding/Account'
import { Libraries } from './onboarding/Libraries'
import { Scan } from './onboarding/Scan'
import { Start } from './onboarding/Start'

// The admin area is its own chunk (F007 NFR bundle).
const AdminApp = lazy(() => import('./admin/routes'))
// The player and hls.js are their own chunk too (F008-R33).
const Player = lazy(() => import('./player/Player'))

export function AppRoutes() {
  return (
    <AuthProvider>
      <Routes>
        <Route
          path="/setup"
          element={
            <RequireStep allow={['setup']}>
              <Start />
            </RequireStep>
          }
        />
        <Route
          path="/setup/account"
          element={
            <RequireStep allow={['setup']}>
              <Account />
            </RequireStep>
          }
        />
        <Route
          path="/setup/libraries"
          element={
            <RequireStep allow={['app']} admin>
              <Libraries />
            </RequireStep>
          }
        />
        <Route
          path="/setup/scan"
          element={
            <RequireStep allow={['app']} admin>
              <Scan />
            </RequireStep>
          }
        />
        <Route
          path="/login"
          element={
            <RequireStep allow={['login']}>
              <SignIn />
            </RequireStep>
          }
        />
        <Route
          path="/change-password"
          element={
            <RequireStep allow={['change-password']}>
              <ChangePassword />
            </RequireStep>
          }
        />
        <Route
          path="/profiles"
          element={
            <RequireStep allow={['profiles', 'app']}>
              <ProfilePicker />
            </RequireStep>
          }
        />
        <Route
          path="/pair"
          element={
            <RequireStep allow={['app']}>
              <Pair />
            </RequireStep>
          }
        />
        {/* The browsing screens share the shell (F008-R1). */}
        <Route
          element={
            <RequireStep allow={['app']}>
              <Shell>
                <Outlet />
              </Shell>
            </RequireStep>
          }
        >
          <Route path="/" element={<HomePage />} />
          <Route path="/library/:libraryId" element={<LibraryGrid />} />
          <Route path="/library/:libraryId/collections" element={<Collections />} />
          <Route path="/collection/:collectionId" element={<CollectionPage />} />
          <Route path="/item/:itemId" element={<ItemPage />} />
          <Route path="/person/:personId" element={<PersonPage />} />
          <Route path="/search" element={<SearchPage />} />
        </Route>
        <Route
          path="/watch/:itemId"
          element={
            <RequireStep allow={['app']}>
              <Suspense>
                <Player />
              </Suspense>
            </RequireStep>
          }
        />
        <Route
          path="/admin/*"
          element={
            <RequireStep allow={['app']} admin>
              <Suspense>
                <AdminApp />
              </Suspense>
            </RequireStep>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AuthProvider>
  )
}

export function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  )
}
