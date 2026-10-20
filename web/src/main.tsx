import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { purgeLegacyBrowserData } from './lib/authSession'

// Drop account data older builds left in browser storage before anything reads it.
void purgeLegacyBrowserData()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
