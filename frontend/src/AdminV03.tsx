import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, Modal, Select, Space, Table, Tabs, Tag, message } from 'antd'
import { EditOutlined, ReloadOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { api } from './api/client'
import { RuntimesV21 } from './DomainManagement'
import { useI18n } from './i18n'
import { EntityMultiSelect, ResponsiveFormGrid, TableActions } from './UxComponents'

function V03PageHeader({ title, subtitle, action }: { title: string; subtitle?: string; action?: React.ReactNode }) {
  return <div className="page-heading"><div><h1 className="page-title">{title}</h1>{subtitle && <div className="page-subtitle">{subtitle}</div>}</div>{action}</div>
}

function V03Status({ value }: { value?: string }) {
  const normalized = value || 'unknown'
  const color = ['active', 'healthy', 'running', 'connected'].includes(normalized) ? 'green' : ['disabled', 'error', 'down', 'failed'].includes(normalized) ? 'red' : 'gold'
  return <Tag color={color}>{normalized.replaceAll('_', ' ')}</Tag>
}

export function ModelsV03() {
  const { t } = useI18n()
  const [models, setModels] = useState<any[]>([])
  const [providers, setProviders] = useState<any[]>([])
  const [providerModels, setProviderModels] = useState<any[]>([])
  const [editing, setEditing] = useState<any>()
  const [open, setOpen] = useState(false)
  const [error, setError] = useState<unknown>()
  const [form] = Form.useForm()
  const providerID = Form.useWatch('provider_id', form)

  const load = async () => {
    try {
      const [modelData, providerData] = await Promise.all([api.get<any[]>('/models'), api.get<any[]>('/model-providers')])
      setModels(modelData)
      setProviders(providerData)
    } catch (cause) { setError(cause) }
  }
  useEffect(() => { void load() }, [])
  useEffect(() => {
    if (!providerID) { setProviderModels([]); return }
    api.get<any[]>(`/provider-models?provider_id=${providerID}`).then(setProviderModels).catch(setError)
  }, [providerID])
  const close = () => { setOpen(false); setEditing(undefined); form.resetFields() }
  const openEditor = (model?: any) => { setEditing(model); form.resetFields(); if (model) form.setFieldsValue(model); setOpen(true) }
  return <>
    <V03PageHeader title={t('models')} subtitle={t('modelBoundary')} action={<Button type="primary" onClick={() => openEditor()}>{t('create')}</Button>} />
    {error && <Card size="small" type="inner" style={{ marginBottom: 16 }}><Tag color="red">{(error as Error).message || t('errorFallback')}</Tag></Card>}
    <Card className="content-card"><Table rowKey="id" dataSource={models} scroll={{ x: 1050 }} columns={[
      { title: t('name'), render: (_: unknown, row: any) => <div><b>{row.display_name}</b><div className="muted">{row.name}</div></div> },
      { title: t('providers'), render: (_: unknown, row: any) => row.provider_name || row.provider || '—' },
      { title: t('upstream'), render: (_: unknown, row: any) => row.provider_model_display || row.upstream_model || '—' },
      { title: t('status'), render: (_: unknown, row: any) => <V03Status value={row.status} /> },
      { title: t('actions'), fixed: 'right', render: (_: unknown, row: any) => <TableActions onEdit={() => openEditor(row)} /> },
    ]} /></Card>
    <Modal title={editing ? t('edit') : t('create')} open={open} onCancel={close} footer={null} width={720}>
      <Form form={form} layout="vertical" onFinish={async (values) => { try { const payload = { ...values, provider: providers.find((item) => item.id === values.provider_id)?.name || values.provider || '', upstream_model: providerModels.find((item) => item.id === values.provider_model_id)?.upstream_model || values.upstream_model || '' }; editing ? await api.put(`/models/${editing.id}`, payload) : await api.post('/models', payload); message.success(t('saved')); close(); await load() } catch (cause) { setError(cause) } }}>
        <ResponsiveFormGrid>
          <Form.Item name="name" label={t('name')} rules={[{ required: !editing }]}><Input disabled={!!editing} /></Form.Item>
          <Form.Item name="display_name" label={t('displayName')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="provider_id" label={t('providers')} rules={[{ required: true }]}><Select showSearch optionFilterProp="label" options={providers.map((item) => ({ value: item.id, label: `${item.name} · ${item.mode}` }))} /></Form.Item>
          <Form.Item name="provider_model_id" label={t('providerModels')}><Select allowClear showSearch optionFilterProp="label" options={providerModels.map((item) => ({ value: item.id, label: `${item.display_name} · ${item.upstream_model}` }))} onChange={(value) => { const selected = providerModels.find((item) => item.id === value); if (selected && !form.getFieldValue('display_name')) form.setFieldValue('display_name', selected.display_name) }} /></Form.Item>
          <Form.Item name="upstream_model" label={t('upstream')}><Input /></Form.Item>
          <Form.Item name="status" label={t('status')} initialValue="active"><Select options={['active', 'disabled'].map((value) => ({ value }))} /></Form.Item>
          <Form.Item name="cost_class" label={t('costClass')}><Input /></Form.Item>
          <Form.Item name="data_classification" label={t('dataClassification')}><Input /></Form.Item>
          <Form.Item name="description" label={t('description')}><Input.TextArea /></Form.Item>
        </ResponsiveFormGrid>
        <Button type="primary" htmlType="submit" block>{t('save')}</Button>
      </Form>
    </Modal>
  </>
}

export function ModelProvidersV03() {
  const { t } = useI18n()
  const [providers, setProviders] = useState<any[]>([])
  const [providerModels, setProviderModels] = useState<any[]>([])
  const [secrets, setSecrets] = useState<any[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<any>()
  const [form] = Form.useForm()
  const [error, setError] = useState<unknown>()
  const load = async () => { try { const [p, m, s] = await Promise.all([api.get<any[]>('/model-providers'), api.get<any[]>('/provider-models'), api.get<any[]>('/secrets')]); setProviders(p); setProviderModels(m); setSecrets(s) } catch (cause) { setError(cause) } }
  useEffect(() => { void load() }, [])
  return <>
    <V03PageHeader title={t('providers')} subtitle={t('modelModes')} action={<Button type="primary" onClick={() => { setEditing(undefined); form.resetFields(); setOpen(true) }}>{t('create')}</Button>} />
    {error && <Card size="small" type="inner" style={{ marginBottom: 16 }}><Tag color="red">{(error as Error).message || t('errorFallback')}</Tag></Card>}
    <Tabs items={[
      { key: 'providers', label: t('providers'), children: <Card className="content-card"><Table rowKey="id" dataSource={providers} scroll={{ x: 1100 }} columns={[
        { title: t('name'), dataIndex: 'name' }, { title: t('type'), dataIndex: 'type' }, { title: t('mode'), dataIndex: 'mode' }, { title: t('baseUrl'), dataIndex: 'base_url' },
        { title: t('secret'), render: (_: unknown, row: any) => <V03Status value={row.secret_status} /> }, { title: t('health'), render: (_: unknown, row: any) => <V03Status value={row.health_status} /> },
        { title: t('actions'), fixed: 'right', render: (_: unknown, row: any) => <Space><Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(row); form.setFieldsValue(row); setOpen(true) }}>{t('edit')}</Button><Button size="small" icon={<SafetyCertificateOutlined />} onClick={async () => { await api.post(`/model-providers/${row.id}/test`, {}); message.success(t('providerTested')); await load() }}>{t('testConnection')}</Button><Button size="small" icon={<ReloadOutlined />} onClick={async () => { await api.post(`/model-providers/${row.id}/sync`, {}); message.success(t('synced')); await load() }}>{t('syncModels')}</Button></Space> },
      ]} /></Card> },
      { key: 'models', label: t('providerModels'), children: <Card className="content-card"><Table rowKey="id" dataSource={providerModels} columns={[{ title: t('providers'), dataIndex: 'provider' }, { title: t('name'), dataIndex: 'display_name' }, { title: t('upstream'), dataIndex: 'upstream_model' }, { title: t('status'), render: (_: unknown, row: any) => <V03Status value={row.status} /> }, { title: t('syncStatus'), dataIndex: 'sync_status' }]} /></Card> },
    ]} />
    <Modal title={editing ? t('edit') : t('providers')} open={open} onCancel={() => { setOpen(false); setEditing(undefined); form.resetFields() }} footer={null} width={720}><Form form={form} layout="vertical" onFinish={async (values) => { try { editing ? await api.put('/model-providers/' + editing.id, values) : await api.post('/model-providers', values); message.success(t(editing ? 'saved' : 'created')); setOpen(false); setEditing(undefined); form.resetFields(); await load() } catch (cause) { setError(cause) } }}><ResponsiveFormGrid><Form.Item name="name" label={t('name')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="type" label={t('type')} initialValue="custom"><Select options={['openai', 'openrouter', 'google', 'nous', 'custom', 'enterprise_gateway'].map((value) => ({ value }))} /></Form.Item><Form.Item name="mode" label={t('mode')} initialValue="hermes_native"><Select options={[{ value: 'hermes_native', label: t('hermesNative') }, { value: 'enterprise_gateway', label: t('enterpriseGateway') }, { value: 'custom_gateway', label: t('customGateway') }]} /></Form.Item><Form.Item name="base_url" label={t('baseUrl')}><Input /></Form.Item><Form.Item name="auth_type" label={t('authType')} initialValue="secret_reference"><Select options={[{ value: 'secret_reference', label: t('secretReference') }, { value: 'reserved', label: t('reserved') }]} /></Form.Item><Form.Item name="secret_reference_id" label={t('secretReference')}><Select allowClear options={secrets.map((item) => ({ value: item.id, label: `${item.name} · ${item.status}` }))} /></Form.Item><Form.Item name="description" label={t('description')}><Input.TextArea /></Form.Item></ResponsiveFormGrid><Button type="primary" htmlType="submit" block>{t('save')}</Button></Form></Modal>
  </>
}

function RuntimeHosts() {
  const { t } = useI18n()
  const [hosts, setHosts] = useState<any[]>([])
  const [secrets, setSecrets] = useState<any[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<any>()
  const [form] = Form.useForm()
  const [error, setError] = useState<unknown>()
  const load = async () => { try { const [h, s] = await Promise.all([api.get<any[]>('/runtime-hosts'), api.get<any[]>('/secrets')]); setHosts(h); setSecrets(s) } catch (cause) { setError(cause) } }
  useEffect(() => { void load() }, [])
  const edit = (host?: any) => { setEditing(host); form.resetFields(); if (host) form.setFieldsValue({ ...host, credential_reference_id: host.credential_reference_id }); setOpen(true) }
  return <><div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 16 }}><Button type="primary" onClick={() => edit()}>{t('create')}</Button></div>{error && <Tag color="red">{(error as Error).message || t('errorFallback')}</Tag>}<Card className="content-card"><Table rowKey="id" dataSource={hosts} scroll={{ x: 1250 }} columns={[{ title: t('name'), render: (_: unknown, row: any) => <div><b>{row.name}</b><div className="muted">{row.hostname}</div></div> }, { title: t('address'), render: (_: unknown, row: any) => `${row.address}:${row.ssh_port}` }, { title: t('capacity'), render: (_: unknown, row: any) => `${row.cpu_total} · ${row.memory_total} · ${row.storage_total}` }, { title: t('runtimeCount'), dataIndex: 'runtime_count' }, { title: t('status'), render: (_: unknown, row: any) => <V03Status value={row.status} /> }, { title: t('actions'), fixed: 'right', render: (_: unknown, row: any) => <Space><Button size="small" icon={<EditOutlined />} onClick={() => edit(row)}>{t('edit')}</Button><Button size="small" onClick={async () => { await api.post(`/runtime-hosts/${row.id}/test`, {}); await load() }}>{t('testConnection')}</Button><Button size="small" onClick={async () => { await api.post(`/runtime-hosts/${row.id}/inventory`, {}); await load() }}>{t('inventory')}</Button></Space> }]} /></Card><Modal title={editing ? t('edit') : t('create')} open={open} onCancel={() => setOpen(false)} footer={null} width={820}><Form form={form} layout="vertical" onFinish={async (values) => { try { editing ? await api.put(`/runtime-hosts/${editing.id}`, values) : await api.post('/runtime-hosts', values); message.success(t('saved')); setOpen(false); await load() } catch (cause) { setError(cause) } }}><ResponsiveFormGrid><Form.Item name="name" label={t('name')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="hostname" label={t('hostname')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="address" label={t('address')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="ssh_port" label={t('sshPort')} initialValue={22}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item><Form.Item name="auth_type" label={t('authType')} initialValue="secret_reference"><Select options={[{ value: 'secret_reference', label: t('secretReference') }, { value: 'reserved', label: t('reserved') }]} /></Form.Item><Form.Item name="credential_reference_id" label={t('credentialReference')}><Select allowClear options={secrets.map((item) => ({ value: item.id, label: `${item.name} · ${item.status}` }))} /></Form.Item><Form.Item name="docker_endpoint" label={t('runtimeEndpoint')} initialValue="mock://local-runtime-provider"><Input /></Form.Item><Form.Item name="cpu_total" label={t('cpu')} initialValue="8 CPU"><Input /></Form.Item><Form.Item name="memory_total" label={t('memory')} initialValue="16 GB"><Input /></Form.Item><Form.Item name="storage_total" label={t('storage')} initialValue="200 GB"><Input /></Form.Item></ResponsiveFormGrid><Button type="primary" htmlType="submit" block>{t('save')}</Button></Form></Modal></>
}

export function RuntimeManagementV03() {
  const { t } = useI18n()
  return <Tabs items={[{ key: 'runtimes', label: t('userRuntimes'), children: <RuntimesV21 /> }, { key: 'hosts', label: t('runtimeHosts'), children: <RuntimeHosts /> }]} />
}
