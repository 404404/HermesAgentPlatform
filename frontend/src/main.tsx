import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import App from './AppV2'
import { I18nProvider } from './i18n'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider theme={{ token: { colorPrimary: '#315efb', borderRadius: 8, colorBgLayout: '#f4f6fb' } }}>
      <BrowserRouter><I18nProvider><App /></I18nProvider></BrowserRouter>
    </ConfigProvider>
  </React.StrictMode>,
)
