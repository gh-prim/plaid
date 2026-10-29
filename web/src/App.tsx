import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { AuthProvider } from './auth/AuthProvider'
import { RequireStep } from './auth/RequireAuth'
import { ChangePassword } from './pages/ChangePassword'
import { Home } from './pages/Home'
import { Pair } from './pages/Pair'
import { ProfilePicker } from './pages/ProfilePicker'
import { SignIn } from './pages/SignIn'
import { Account } from './onboarding/Account'
import { Libraries } from './onboarding/Libraries'
import { Scan } from './onboarding/Scan'
import { Start } from './onboarding/Start'

// The admin area is its own chunk (F007 NFR bundle).
const AdminApp = lazy(() => import('./admin/routes'))

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
        <Route
          path="/"
          element={
            <RequireStep allow={['app']}>
              <Home />
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
