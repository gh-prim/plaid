import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { AuthProvider } from './auth/AuthProvider'
import { RequireStep } from './auth/RequireAuth'
import { ChangePassword } from './pages/ChangePassword'
import { Home } from './pages/Home'
import { Pair } from './pages/Pair'
import { ProfilePicker } from './pages/ProfilePicker'
import { Setup } from './pages/Setup'
import { SignIn } from './pages/SignIn'

// Development only: the design system's components (F007 PR A).
const Gallery = import.meta.env.DEV
  ? lazy(() => import('./dev/Gallery').then((m) => ({ default: m.Gallery })))
  : null

export function AppRoutes() {
  return (
    <AuthProvider>
      <Routes>
        <Route
          path="/setup"
          element={
            <RequireStep allow={['setup']}>
              <Setup />
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
        <Route
          path="/"
          element={
            <RequireStep allow={['app']}>
              <Home />
            </RequireStep>
          }
        />
        {Gallery && (
          <Route
            path="/dev/ui"
            element={
              <Suspense>
                <Gallery />
              </Suspense>
            }
          />
        )}
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
