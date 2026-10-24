import { Layout } from 'antd'
import Workspace from './pages/Workspace'

export default function App() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Content style={{ maxWidth: 1280, width: '100%', margin: '0 auto', padding: 24 }}>
        <Workspace />
      </Layout.Content>
    </Layout>
  )
}
