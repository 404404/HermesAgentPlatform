import { Button, Dropdown, message, Modal, Space, Switch, Tooltip } from 'antd'
import { EditOutlined, EyeOutlined, MoreOutlined } from '@ant-design/icons'
import { useI18n } from './i18n'

// Standard compact, sticky-table action cell. Page-level actions remain
// primary buttons; destructive actions are intentionally kept in More.
export function ActionCell({
  onView, onEdit, enabled, onEnabledChange, loading, moreItems,
}: {
  onView?: () => void
  onEdit?: () => void
  enabled?: boolean
  onEnabledChange?: (next: boolean) => void
  loading?: boolean
  moreItems?: Array<{ key: string; label: string; danger?: boolean; onClick: () => void }>
}) {
  const { t } = useI18n()
  const invoke = (item: NonNullable<typeof moreItems>[number]) => () => { void Promise.resolve(item.onClick()).catch(() => message.error(t("errorFallback"))) }
  const items = moreItems?.map((item) => ({ ...item, onClick: item.danger ? () => Modal.confirm({ title: t("confirm"), content: t("dangerousActionConfirm"), okButtonProps: { danger: true }, onOk: invoke(item) }) : invoke(item) }))
  return <Space className="table-actions" size={4}>
    {onEnabledChange && <Tooltip title={t('status')}><Switch size="small" checked={enabled} loading={loading} onChange={(next) => { void Promise.resolve(onEnabledChange(next)).catch(() => message.error(t("errorFallback"))) }} /></Tooltip>}
    {onView && <Tooltip title={t('viewDetails')}><Button size="small" type="text" icon={<EyeOutlined />} onClick={onView} aria-label={t('viewDetails')} /></Tooltip>}
    {onEdit && <Tooltip title={t('edit')}><Button size="small" type="text" icon={<EditOutlined />} onClick={onEdit} aria-label={t('edit')} /></Tooltip>}
    {moreItems?.length ? <Dropdown menu={{ items }} trigger={['click']}><Tooltip title={t('moreActions')}><Button size="small" type="text" icon={<MoreOutlined />} aria-label={t('moreActions')} /></Tooltip></Dropdown> : null}
  </Space>
}
