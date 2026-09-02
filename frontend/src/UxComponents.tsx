import { ReactNode } from 'react'
import { Button, Dropdown, Popover, Select, Space, Tag } from 'antd'
import { DownOutlined, EditOutlined, EyeOutlined, MoreOutlined, SearchOutlined } from '@ant-design/icons'

export type EntityOption = {
  value: string | number
  label: string
  description?: string
  category?: string
  risk?: string
  status?: string
  disabled?: boolean
}

export function EntityMultiSelect({
  options,
  value,
  onChange,
  placeholder = 'Search and select',
  disabled,
  loading,
}: {
  options: EntityOption[]
  value?: (string | number)[]
  onChange?: (value: (string | number)[]) => void
  placeholder?: string
  disabled?: boolean
  loading?: boolean
}) {
  return <Select
    mode="multiple"
    showSearch
    allowClear
    loading={loading}
    disabled={disabled}
    value={value}
    onChange={onChange}
    optionFilterProp="label"
    suffixIcon={<SearchOutlined />}
    placeholder={placeholder}
    style={{ width: '100%' }}
    options={options.map((option) => ({
      value: option.value,
      label: option.label,
      disabled: option.disabled,
      title: [option.label, option.category, option.risk].filter(Boolean).join(' · '),
      optionRender: () => <Space direction="vertical" size={0}><span>{option.label}</span><small className="muted">{[option.category, option.risk, option.status, option.description].filter(Boolean).join(' · ')}</small></Space>,
    }))}
  />
}

export function RelationTags({
  values,
  max = 3,
  onClick,
}: {
  values?: Array<string | { id?: string | number; name?: string; label?: string }>
  max?: number
  onClick?: (value: string | { id?: string | number; name?: string; label?: string }) => void
}) {
  const normalized = (values || []).map((value) => typeof value === 'string' ? value : (value.name || value.label || String(value.id || '—')))
  if (!normalized.length) return <span className="muted">—</span>
  const visible = normalized.slice(0, max)
  const rest = normalized.slice(max)
  return <Space size={[4, 4]} wrap>
    {visible.map((value, index) => <Tag key={`${value}-${index}`} className={onClick ? 'relation-tag-clickable' : undefined} onClick={() => onClick?.((values || [])[index])}>{value}</Tag>)}
    {rest.length > 0 && <Popover title="All related entities" content={<Space direction="vertical">{rest.map((value, index) => <Tag key={`${value}-${index}`} className={onClick ? 'relation-tag-clickable' : undefined} onClick={() => onClick?.((values || [])[max + index])}>{value}</Tag>)}</Space>}><Tag color="blue">+{rest.length}<DownOutlined /></Tag></Popover>}
  </Space>
}

export function ResponsiveFormGrid({ children }: { children: ReactNode }) {
  return <div className="responsive-form-grid">{children}</div>
}

export function TableActions({
  onView,
  onEdit,
  moreItems,
}: {
  onView?: () => void
  onEdit?: () => void
  moreItems?: Array<{ key: string; label: string; danger?: boolean; onClick: () => void }>
}) {
  const items = moreItems?.map(({ onClick, ...item }) => ({ ...item, onClick }))
  return <Space className="table-actions" size={4}>
    {onView && <Button size="small" type="text" icon={<EyeOutlined />} onClick={onView}>View</Button>}
    {onEdit && <Button size="small" type="text" icon={<EditOutlined />} onClick={onEdit}>Edit</Button>}
    {items?.length ? <Dropdown menu={{ items }} trigger={['click']}><Button size="small" type="text" icon={<MoreOutlined />} aria-label="More actions" /></Dropdown> : null}
  </Space>
}
