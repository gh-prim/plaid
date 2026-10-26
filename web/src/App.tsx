import { BrowserRouter, Route, Routes } from 'react-router'
import { Welcome } from './pages/Welcome'

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="*" element={<Welcome />} />
      </Routes>
    </BrowserRouter>
  )
}
