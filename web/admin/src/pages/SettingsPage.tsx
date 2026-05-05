import { Card, Form, Input, Button, message } from 'antd'
import { SaveOutlined } from '@ant-design/icons'

export default function SettingsPage() {
  const [form] = Form.useForm()

  const onFinish = (values: any) => {
    console.log('Settings:', values)
    message.success('设置已保存')
  }

  return (
    <div>
      <h2>系统设置</h2>

      <Card title="站点设置" style={{ marginBottom: 24 }}>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{
            site_name: 'Go Agent Platform',
            site_description: 'Local-first Agent Studio',
          }}
        >
          <Form.Item
            name="site_name"
            label="站点名称"
            rules={[{ required: true, message: '请输入站点名称' }]}
          >
            <Input />
          </Form.Item>

          <Form.Item name="site_description" label="站点描述">
            <Input.TextArea rows={3} />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />}>
              保存设置
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card title="注册设置">
        <Form layout="vertical">
          <Form.Item
            name="registration_enabled"
            label="允许注册"
            valuePropName="checked"
          >
            <Input type="checkbox" />
          </Form.Item>

          <Form.Item
            name="max_upload_size"
            label="最大上传文件大小 (MB)"
          >
            <Input type="number" />
          </Form.Item>

          <Form.Item>
            <Button type="primary" icon={<SaveOutlined />}>
              保存设置
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
