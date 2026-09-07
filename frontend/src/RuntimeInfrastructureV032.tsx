import { useEffect, useState } from 'react'
import { Button, Card, Descriptions, Drawer, Form, Input, InputNumber, Modal, Select, Table, Tabs, Tag, message } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { api } from './api/client'
import { useI18n } from './i18n'
import { ActionCell } from './ActionCell'
import { ResponsiveFormGrid } from './UxComponents'

const hostStatus = ['online', 'offline', 'degraded', 'maintenance', 'draining', 'unknown']

function HostStatus({ value }: { value?: string }) {
  const { t } = useI18n()
  const normalized = value || 'unknown'
  return <Tag color={normalized === 'online' ? 'green' : normalized === 'offline' ? 'red' : 'gold'}>{t(`enum.${normalized}`, normalized)}</Tag>
}

export function RuntimeInfrastructureV032() {
  const { t } = useI18n()
  const [hosts, setHosts] = useState<any[]>([])
  const [error, setError] = useState<unknown>()
  const [open, setOpen] = useState(false)
  const [detail, setDetail] = useState<any>()
  const [editing, setEditing] = useState<any>()
  const [form] = Form.useForm()
  const load = async () => { try { setHosts(await api.get<any[]>('/runtime-hosts')) } catch (cause) { setError(cause) } }
  useEffect(() => { void load() }, [])
  const edit = (host?: any) => { setEditing(host); form.resetFields(); form.setFieldsValue(host || { ssh_port: 22, auth_type: 'password', docker_socket_path: '/var/run/docker.sock', docker_binary: 'docker', status: 'unknown' }); setOpen(true) }
  const save = async (values: any) => {
    try { editing ? await api.put(`/runtime-hosts/${editing.id}`, values) : await api.post('/runtime-hosts', values); message.success(t('saved')); setOpen(false); await load() } catch (cause) { setError(cause) }
  }
  const openDetail = async (host: any) => { try { setDetail(await api.get<any>(`/runtime-hosts/${host.id}`)) } catch (cause) { setError(cause) } }
  const test = async (host: any) => { try { await api.post(`/runtime-hosts/${host.id}/test`, {}); message.success(t('connectionTested')); await load() } catch (cause) { setError(cause) } }
  const inventory = async (host: any) => { try { await api.post(`/runtime-hosts/${host.id}/inventory`, {}); message.success(t('inventoryUpdated')); await load() } catch (cause) { setError(cause) } }
  const setStatus = async (host: any, status: string) => { try { await api.post(`/runtime-hosts/${host.id}/status`, { status }); await load() } catch (cause) { setError(cause) } }
  return <Card className="content-card" title={t('runtimeInfrastructure')} extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => edit()}>{t('addRuntimeHost')}</Button>}>
    {!!error && <Tag color="red">{(error as Error).message || t('errorFallback')}</Tag>}
    <Tabs items={[
      { key: 'hosts', label: t('runtimeHosts'), children: <Table rowKey="id" dataSource={hosts} scroll={{ x: 1450 }} columns={[
        { title: t('name'), render: (_: unknown, row: any) => <Button type="link" onClick={() => openDetail(row)}>{row.name}</Button> },
        { title: t('hostIp'), render: (_: unknown, row: any) => `${row.address} · ${row.hostname}` }, { title: t('sshPort'), dataIndex: 'ssh_port' },
        { title: t('authentication'), render: (_: unknown, row: any) => row.credential_reference_configured ? t('configured') : t('notConfigured') },
        { title: 'Docker', dataIndex: 'docker_version' }, { title: t('cpu'), dataIndex: 'cpu_total' }, { title: t('memory'), dataIndex: 'memory_total' }, { title: t('storage'), dataIndex: 'storage_total' },
        { title: t('userRuntimes'), dataIndex: 'runtime_count' }, { title: t('status'), render: (_: unknown, row: any) => <HostStatus value={row.status} /> }, { title: t('lastCheck'), dataIndex: 'last_seen' },
        { title: t('actions'), fixed: 'right', render: (_: unknown, row: any) => <ActionCell onView={() => openDetail(row)} onEdit={() => edit(row)} enabled={row.status === 'online'} onEnabledChange={(enabled) => setStatus(row, enabled ? 'online' : 'maintenance')} moreItems={[{ key: 'test', label: t('testConnection'), onClick: () => test(row) }, { key: 'inventory', label: t('inventory'), onClick: () => inventory(row) }, { key: 'drain', label: t('drain'), onClick: () => setStatus(row, 'draining') }, { key: 'delete', label: t('delete'), danger: true, onClick: async () => { await api.delete(`/runtime-hosts/${row.id}`); await load() } }]} /> },
      ]} /> },
      { key: 'defaults', label: t('runtimeDefaults'), children: <Descriptions column={2} items={[{ key: 'template', label: t('defaultTemplate'), children: t('configured') }, { key: 'provider', label: t('defaultRuntimeProvider'), children: 'MockRuntimeProvider' }, { key: 'image', label: t('defaultHermesImage'), children: 'hermes:demo' }, { key: 'socket', label: t('defaultDockerSocket'), children: '/var/run/docker.sock' }, { key: 'timeout', label: t('provisionTimeout'), children: '10 min' }, { key: 'interval', label: t('healthCheckInterval'), children: '60 sec' }]} /> },
      { key: 'scheduling', label: t('scheduling'), children: <Descriptions column={1} items={[{ key: 'mode', label: t('placementMode'), children: t('automatic') }, { key: 'rule', label: t('placementRule'), children: t('mockSchedulingRule') }]} /> },
    ]} />
    <Modal title={editing ? t('editRuntimeHost') : t('addRuntimeHost')} open={open} onCancel={() => setOpen(false)} footer={null} width={960} destroyOnClose>
      <Form form={form} layout="vertical" onFinish={save}><ResponsiveFormGrid>
        <Form.Item name="name" label={t('hostName')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="hostname" label={t('hostname')} rules={[{ required: true }]}><Input /></Form.Item>
        <Form.Item name="address" label={t('hostIp')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="ssh_port" label={t('sshPort')}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="ssh_username" label={t('username')}><Input /></Form.Item><Form.Item name="auth_type" label={t('authentication')}><Select options={['password', 'ssh_key', 'certificate', 'agent'].map((value) => ({ value, label: value === 'password' ? t('sshPassword') : `${value} · ${t('reserved')}`, disabled: value !== 'password' }))} /></Form.Item>
        <Form.Item name="credential" label={t('sshPassword')} extra={editing?.credential_reference_configured ? t('credentialConfiguredHint') : t('credentialRequiredHint')}><Input.Password autoComplete="new-password" placeholder={editing?.credential_reference_configured ? t('leaveBlankUnchanged') : undefined} /></Form.Item><Form.Item name="docker_socket_path" label={t('dockerSocketPath')}><Input /></Form.Item>
        <Form.Item name="docker_binary" label={t('dockerBinary')}><Input /></Form.Item><Form.Item name="status" label={t('status')}><Select options={hostStatus.map((value) => ({ value, label: t(`enum.${value}`, value) }))} /></Form.Item>
        <Form.Item name="cpu_total" label={t('cpu')}><Input /></Form.Item><Form.Item name="memory_total" label={t('memory')}><Input /></Form.Item><Form.Item name="storage_total" label={t('storage')}><Input /></Form.Item>
        <Form.Item name="labels" label={t('labels')}><Select mode="tags" /></Form.Item><Form.Item name="description" label={t('description')} className="span-2"><Input.TextArea rows={3} /></Form.Item>
      </ResponsiveFormGrid><Button type="primary" htmlType="submit" block>{t('save')}</Button></Form>
    </Modal>
    <Drawer title={detail?.name || t('runtimeHost')} open={!!detail} onClose={() => setDetail(undefined)} width={760}>{detail && <Tabs items={[
      { key: 'overview', label: t('overviewTab'), children: <Descriptions column={2} items={[{ key: 'address', label: t('hostIp'), children: detail.address }, { key: 'status', label: t('status'), children: <HostStatus value={detail.status} /> }, { key: 'auth', label: t('authentication'), children: detail.credential_reference_configured ? t('configured') : t('notConfigured') }, { key: 'docker', label: 'Docker', children: detail.docker_version }]} /> },
      { key: 'resources', label: t('resources'), children: <Descriptions column={2} items={['cpu', 'memory', 'storage'].flatMap((name) => [{ key: `${name}Total`, label: `${t(name)} ${t('total')}`, children: detail[`${name}_total`] }, { key: `${name}Allocated`, label: `${t(name)} ${t('allocated')}`, children: detail[`${name}_allocated`] }, { key: `${name}Actual`, label: `${t(name)} ${t('actualUsage')}`, children: detail[`${name}_actual`] }])} /> },
      { key: 'runtimes', label: t('userRuntimes'), children: <Table rowKey="id" size="small" dataSource={detail.runtimes || []} columns={[{ title: t('name'), dataIndex: 'runtime_id' }, { title: t('owner'), dataIndex: 'user_name' }, { title: t('status'), dataIndex: 'status' }]} /> },
      { key: 'containers', label: t('containers'), children: <Tag>{t('mockProviderBoundary')}</Tag> }, { key: 'connection', label: t('connection'), children: <Descriptions column={1} items={[{ key: 'socket', label: t('dockerSocketPath'), children: detail.docker_socket_path }, { key: 'binary', label: t('dockerBinary'), children: detail.docker_binary }]} /> }, { key: 'events', label: t('events'), children: <Tag>{t('mockProviderBoundary')}</Tag> },
    ]} />}</Drawer>
  </Card>
}
