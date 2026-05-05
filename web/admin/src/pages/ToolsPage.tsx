import { useEffect, useState } from 'react'
import { Table, Card, Button, Tag, Space, message, Popconfirm } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { toolsApi, type ToolItem } from '../api'

const scopeColors: Record<string, string> = {
  platform: 'blue',
  personal: 'green',
}

const statusColors: Record<string, string> = {
  draft: 'default',
  published: 'success',
  archived: 'warning',
}

export default function ToolsPage() {
  const [tools, setTools] = useState<ToolItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)

  useEffect(() => {
    loadTools()
  }, [page])

  const loadTools = async () => {
    setLoading(true)
    try {
      const { data } = await toolsApi.list({ page, page_size: 10 })
      setTools(data.items || [])
      setTotal(data.total || 0)
    } catch (error) {
      message.error('加载 MCP 工具列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await toolsApi.delete(id)
      message.success('删除成功')
      loadTools()
    } catch (error) {
      message.error('删除失败')
    }
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 120,
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Slug',
      dataIndex: 'slug',
      key: 'slug',
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
    },
    {
      title: '范围',
      dataIndex: 'scope',
      key: 'scope',
      render: (scope: string) => (
        <Tag color={scopeColors[scope] || 'default'}>{scope}</Tag>
      ),
    },
    {
      title: '下载次数',
      dataIndex: 'download_count',
      key: 'download_count',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={statusColors[status] || 'default'}>{status}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: ToolItem) => (
        <Space>
          <Button type="link">编辑</Button>
          <Popconfirm
            title="确定删除此 MCP 工具？"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button type="link" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Card
        title="MCP 工具管理"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadTools}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />}>
              添加 MCP 工具
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={tools}
          rowKey="id"
          loading={loading}
          pagination={{
            current: page,
            total,
            pageSize: 10,
            onChange: setPage,
          }}
        />
      </Card>
    </div>
  )
}
