import { useEffect, useState } from 'react'
import { Table, Card, Button, Tag, Space, message, Popconfirm } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { skillsApi, type SkillItem } from '../api'

const scopeColors: Record<string, string> = {
  platform: 'blue',
  personal: 'green',
}

const statusColors: Record<string, string> = {
  draft: 'default',
  published: 'success',
  archived: 'warning',
}

export default function SkillsPage() {
  const [skills, setSkills] = useState<SkillItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)

  useEffect(() => {
    loadSkills()
  }, [page])

  const loadSkills = async () => {
    setLoading(true)
    try {
      const { data } = await skillsApi.list({ page, page_size: 10 })
      setSkills(data.items || [])
      setTotal(data.total || 0)
    } catch (error) {
      message.error('加载 Skill 列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await skillsApi.delete(id)
      message.success('删除成功')
      loadSkills()
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
      render: (_: any, record: SkillItem) => (
        <Space>
          <Button type="link">编辑</Button>
          <Popconfirm
            title="确定删除此 Skill？"
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
        title="Skill 管理"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadSkills}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />}>
              添加 Skill
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={skills}
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
