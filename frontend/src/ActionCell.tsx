import { Button, Dropdown, message, Modal, Space, Switch, Tooltip } from 'antd'
import { DownOutlined } from '@ant-design/icons'
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
  const stop = (callback: () => void) => (event: any) => { event.stopPropagation(); callback() }
  return <Space className="table-actions" size={2}>
    {onEnabledChange && <Tooltip title={t('status')}><span onClick={(event) => event.stopPropagation()}><Switch size="small" checked={enabled} loading={loading} onChange={(next) => { void Promise.resolve(onEnabledChange(next)).catch(() => message.error(t("errorFallback"))) }} /></span></Tooltip>}
    {onView && <Button size="small" type="text" onClick={stop(onView)}>{t('view')}</Button>}
    {onEdit && <Button size="small" type="text" onClick={stop(onEdit)}>{t('edit')}</Button>}
    {moreItems?.length ? <Dropdown menu={{ items }} trigger={['click']}><Button size="small" type="text" onClick={(event) => event.stopPropagation()}>{t('more')} <DownOutlined /></Button></Dropdown> : null}
  </Space>
}
