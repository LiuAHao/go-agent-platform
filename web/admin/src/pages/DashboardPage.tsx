import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Spin } from 'antd'
import {
  UserOutlined,
  AppstoreOutlined,
  ToolOutlined,
  MessageOutlined,
} from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import { statsApi, type StatsOverview, type DailyStats } from '../api'

export default function DashboardPage() {
  const [overview, setOverview] = useState<StatsOverview | null>(null)
  const [daily, setDaily] = useState<DailyStats[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    try {
      const [overviewRes, dailyRes] = await Promise.all([
        statsApi.overview(),
        statsApi.daily({ days: 7 }),
      ])
      setOverview(overviewRes.data)
      setDaily(dailyRes.data)
    } catch (error) {
      console.error('Failed to load stats:', error)
    } finally {
      setLoading(false)
    }
  }

  const chartOption = {
    tooltip: {
      trigger: 'axis' as const,
    },
    legend: {
      data: ['新用户', '新会话', '新消息'],
    },
    xAxis: {
      type: 'category' as const,
      data: daily.map((d) => d.date),
    },
    yAxis: {
      type: 'value' as const,
    },
    series: [
      {
        name: '新用户',
        type: 'line',
        data: daily.map((d) => d.new_users),
      },
      {
        name: '新会话',
        type: 'line',
        data: daily.map((d) => d.new_sessions),
      },
      {
        name: '新消息',
        type: 'line',
        data: daily.map((d) => d.new_messages),
      },
    ],
  }

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
      </div>
    )
  }

  return (
    <div>
      <h2>仪表盘</h2>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="总用户数"
              value={overview?.total_users || 0}
              prefix={<UserOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="总 Skill 数"
              value={overview?.total_skills || 0}
              prefix={<AppstoreOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="总 MCP 工具数"
              value={overview?.total_tools || 0}
              prefix={<ToolOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="总消息数"
              value={overview?.total_messages || 0}
              prefix={<MessageOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Card title="最近 7 天趋势">
        <ReactECharts option={chartOption} style={{ height: 400 }} />
      </Card>
    </div>
  )
}
