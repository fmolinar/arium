import { Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import Home from './pages/Home'
import NewsHub from './pages/NewsHub'
import Forum from './pages/Forum'
import About from './pages/About'

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Home />} />
        <Route path="news" element={<NewsHub />} />
        <Route path="forum" element={<Forum />} />
        <Route path="about" element={<About />} />
      </Route>
    </Routes>
  )
}

export default App
