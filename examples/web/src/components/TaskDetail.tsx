import React from 'react';
import { Card, Descriptions, Tag, Button, Space, Divider } from 'antd';
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons';
import type { TaskInfo } from '../types/task';

interface TaskDetailProps {
  task: TaskInfo | null;
  onStopTask?: (taskId: number) => void;
  onRefresh?: () => void;
}

const TaskDetail: React.FC<TaskDetailProps> = ({ task, onStopTask, onRefresh }) => {
  if (!task) {
    return (
      <Card title="任务详情">
        <div style={{ textAlign: 'center', padding: '50px' }}>
          请选择一个任务查看详情
        </div>
      </Card>
    );
  }

  const getStateColor = (state: string) => {
    const colors: Record<string, string> = {
      INIT: 'default',
      STARTING: 'processing',
      STARTED: 'processing',
      RUNNING: 'success',
      GOING: 'success',
      DISPOSING: 'warning',
      DISPOSED: 'error',
    };
    return colors[state] || 'default';
  };

  const getTypeColor = (type: string) => {
    const colors: Record<string, string> = {
      TASK: 'blue',
      JOB: 'green',
      WORK: 'orange',
      CHANNEL: 'purple',
    };
    return colors[type] || 'default';
  };

  const formatDuration = (startTime: string) => {
    const start = new Date(startTime);
    const now = new Date();
    const diff = now.getTime() - start.getTime();
    
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}天 ${hours % 24}小时`;
    if (hours > 0) return `${hours}小时 ${minutes % 60}分钟`;
    if (minutes > 0) return `${minutes}分钟 ${seconds % 60}秒`;
    return `${seconds}秒`;
  };

  return (
    <Card 
      title="任务详情" 
      extra={
        <Space>
          {task.state !== 'DISPOSED' && (
            <Button 
              danger 
              icon={<DeleteOutlined />}
              onClick={() => onStopTask?.(task.id)}
            >
              停止任务
            </Button>
          )}
          <Button 
            icon={<ReloadOutlined />} 
            onClick={onRefresh}
          >
            刷新
          </Button>
        </Space>
      }
    >
      <Descriptions column={2} bordered>
        <Descriptions.Item label="任务ID">{task.id}</Descriptions.Item>
        <Descriptions.Item label="任务类型">
          <Tag color={getTypeColor(task.type)}>{task.type}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="拥有者类型">{task.owner}</Descriptions.Item>
        <Descriptions.Item label="任务状态">
          <Tag color={getStateColor(task.state)}>{task.state}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="任务层级">{task.level}</Descriptions.Item>
        <Descriptions.Item label="重试次数">
          {task.retryCount} / {task.maxRetry === -1 ? '∞' : task.maxRetry}
        </Descriptions.Item>
        <Descriptions.Item label="开始时间">
          {new Date(task.startTime).toLocaleString()}
        </Descriptions.Item>
        <Descriptions.Item label="运行时长">
          {formatDuration(task.startTime)}
        </Descriptions.Item>
        <Descriptions.Item label="开始原因" span={2}>
          {task.startReason}
        </Descriptions.Item>
        {task.stopReason && (
          <Descriptions.Item label="停止原因" span={2}>
            <Tag color="error">{task.stopReason}</Tag>
          </Descriptions.Item>
        )}
        {task.parentId && (
          <Descriptions.Item label="父任务ID">{task.parentId}</Descriptions.Item>
        )}
        {task.children && task.children.length > 0 && (
          <Descriptions.Item label="子任务数量">{task.children.length}</Descriptions.Item>
        )}
      </Descriptions>

      {Object.keys(task.description).length > 0 && (
        <>
          <Divider />
          <Descriptions title="描述信息" column={2} bordered>
            {Object.entries(task.description).map(([key, value]) => (
              <Descriptions.Item key={key} label={key}>
                {value}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </>
      )}
    </Card>
  );
};

export default TaskDetail;