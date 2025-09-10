import React, { useState, useEffect } from "react";
import { Table, Card, Button, Space, Tag, message } from "antd";
import {
  DeleteOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import type { TaskTree, TaskInfo } from "../types/task";
import { taskApi } from "../services/api";
import type { ColumnsType } from "antd/es/table";

const TaskTreeComponent: React.FC = () => {
  const [taskTree, setTaskTree] = useState<TaskTree | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchTaskTree = async () => {
    setLoading(true);
    try {
      const data = await taskApi.getTaskTree();
      console.log("获取到的任务树数据:", data);
      // API 直接返回任务对象，而不是 { root: TaskInfo } 格式
      setTaskTree({ root: data });
    } catch (error) {
      console.error("获取任务树失败:", error);
      message.error("获取任务树失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTaskTree();
    const interval = setInterval(fetchTaskTree, 2000);
    return () => clearInterval(interval);
  }, []);

  const handleCreateDemoTask = async () => {
    try {
      await taskApi.createDemoTask();
      message.success("示例任务已创建");
      fetchTaskTree();
    } catch (error) {
      message.error("创建示例任务失败");
    }
  };

  const getStateColor = (state: string) => {
    const colors: Record<string, string> = {
      INIT: "default",
      STARTING: "processing",
      STARTED: "processing",
      RUNNING: "success",
      GOING: "success",
      DISPOSING: "warning",
      DISPOSED: "error",
    };
    return colors[state] || "default";
  };

  const getTypeIcon = (type: string) => {
    const icons: Record<string, React.ReactNode> = {
      TASK: <PlayCircleOutlined />,
      JOB: <DeleteOutlined />,
      WORK: <ReloadOutlined />,
      CHANNEL: <PauseCircleOutlined />,
    };
    return icons[type] || <PlayCircleOutlined />;
  };

  const getTypeColor = (type: string) => {
    const colors: Record<string, string> = {
      TASK: "blue",
      JOB: "green",
      WORK: "orange",
      CHANNEL: "purple",
    };
    return colors[type] || "default";
  };

  // 将树形数据扁平化为表格数据，添加层级信息
  const flattenTaskTree = (
    node: TaskInfo,
    level: number = 0
  ): (TaskInfo & { displayLevel: number })[] => {
    const result: (TaskInfo & { displayLevel: number })[] = [];
    const nodeWithLevel = { ...node, displayLevel: level };
    result.push(nodeWithLevel);

    if (node.children && node.children.length > 0) {
      for (const child of node.children) {
        result.push(...flattenTaskTree(child, level + 1));
      }
    }

    return result;
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

  const columns: ColumnsType<TaskInfo> = [
    {
      title: "拥有者",
      dataIndex: "ownerType",
      key: "ownerType",
      width: 200,
      render: (owner: string, record) => {
        const indent = "  ".repeat(record.level);
        return indent + owner;
      },
    },
    {
      title: "任务ID",
      dataIndex: "id",
      key: "id",
      width: 80,
      sorter: (a, b) => a.id - b.id,
    },
    {
      title: "类型",
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type: string) => (
        <Space>
          {getTypeIcon(type)}
          <Tag color={getTypeColor(type)}>{type}</Tag>
        </Space>
      ),
      filters: [
        { text: "TASK", value: "TASK" },
        { text: "JOB", value: "JOB" },
        { text: "WORK", value: "WORK" },
        { text: "CHANNEL", value: "CHANNEL" },
      ],
      onFilter: (value, record) => record.type === value,
    },
    {
      title: "状态",
      dataIndex: "state",
      key: "state",
      width: 100,
      render: (state: string) => (
        <Tag color={getStateColor(state)}>{state}</Tag>
      ),
      filters: [
        { text: "INIT", value: "INIT" },
        { text: "STARTING", value: "STARTING" },
        { text: "STARTED", value: "STARTED" },
        { text: "RUNNING", value: "RUNNING" },
        { text: "GOING", value: "GOING" },
        { text: "DISPOSING", value: "DISPOSING" },
        { text: "DISPOSED", value: "DISPOSED" },
      ],
      onFilter: (value, record) => record.state === value,
    },
    {
      title: "开始时间",
      dataIndex: "startTime",
      key: "startTime",
      width: 180,
      render: (time: string) => new Date(time).toLocaleString(),
      sorter: (a, b) =>
        new Date(a.startTime).getTime() - new Date(b.startTime).getTime(),
    },
    {
      title: "运行时长",
      key: "duration",
      width: 120,
      render: (_, record) => formatDuration(record.startTime),
    },
    {
      title: "重试次数",
      dataIndex: "retryCount",
      key: "retryCount",
      width: 100,
      render: (retryCount: number, record) =>
        `${retryCount} / ${record.maxRetry === -1 ? "∞" : record.maxRetry}`,
      sorter: (a, b) => a.retryCount - b.retryCount,
    },
    {
      title: "开始原因",
      dataIndex: "startReason",
      key: "startReason",
      width: 200,
      ellipsis: true,
    },
    {
      title: "停止原因",
      dataIndex: "stopReason",
      key: "stopReason",
      width: 150,
      render: (reason: string) =>
        reason ? (
          <Tag color="error">
            {reason.length > 20 ? reason.substring(0, 20) + "..." : reason}
          </Tag>
        ) : (
          "-"
        ),
    },
  ];

  return (
    <Card
      title="任务树"
      style={{ width: "100%", maxWidth: "none" }}
      extra={
        <Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreateDemoTask}
          >
            创建示例任务
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={fetchTaskTree}
            loading={loading}
          >
            刷新
          </Button>
        </Space>
      }
    >
      {taskTree && taskTree.root ? (
        <Table
          columns={columns}
          dataSource={flattenTaskTree(taskTree.root)}
          loading={loading}
          rowKey="id"
          scroll={{ x: 1400 }}
          pagination={{
            pageSize: 50,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条任务`,
          }}
          size="small"
        />
      ) : (
        <div style={{ textAlign: "center", padding: "50px" }}>暂无任务数据</div>
      )}
    </Card>
  );
};

export default TaskTreeComponent;
