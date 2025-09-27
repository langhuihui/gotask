import React, { useState, useEffect } from "react";
import { Table, Card, Button, Space, Tag, message } from "antd";
import {
  PauseCircleOutlined,
  ReloadOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import type { TaskTree, TaskInfo } from "../types/task";
import { taskApi } from "../services/api";
import type { ColumnsType } from "antd/es/table";
import Logo from "./Logo";
import { useLanguage } from "../hooks/useLanguage";

const TaskTreeComponent: React.FC = () => {
  const { t } = useLanguage();
  const [taskTree, setTaskTree] = useState<TaskTree | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchTaskTree = async () => {
    setLoading(true);
    try {
      const data = await taskApi.getTaskTree();
      console.log("获取到的任务树数据:", data);
      // API 直接返回任务对象，需要包装成 { root: TaskInfo } 格式
      setTaskTree({ root: data });
    } catch (error) {
      console.error("获取任务树失败:", error);
      message.error(t("message.getTaskTreeFailed"));
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
      message.success(t("message.createDemoTaskSuccess"));
      fetchTaskTree();
    } catch (error) {
      message.error(t("message.createDemoTaskFailed"));
    }
  };

  const handleRestartTask = async (taskId: number) => {
    try {
      // 这里需要调用重启API，暂时用停止+重新创建来模拟
      await taskApi.stopTask(taskId, t("task.restart"));
      message.success(t("message.taskRestarted"));
      fetchTaskTree();
    } catch (error) {
      message.error(t("message.restartTaskFailed"));
    }
  };

  const handleStopTask = async (taskId: number) => {
    try {
      await taskApi.stopTask(taskId, t("task.stop"));
      message.success(t("message.taskStopped"));
      fetchTaskTree();
    } catch (error) {
      message.error(t("message.stopTaskFailed"));
    }
  };

  const getStateColor = (state: number) => {
    const colors: Record<number, string> = {
      0: "default", // INIT
      1: "processing", // STARTING
      2: "processing", // STARTED
      3: "success", // RUNNING
      4: "success", // GOING
      5: "warning", // DISPOSING
      6: "error", // DISPOSED
    };
    return colors[state] || "default";
  };

  const getStateText = (state: number) => {
    const stateMap: Record<number, string> = {
      0: t("taskState.init"), // INIT
      1: t("taskState.starting"), // STARTING
      2: t("taskState.started"), // STARTED
      3: t("taskState.running"), // RUNNING
      4: t("taskState.going"), // GOING
      5: t("taskState.disposing"), // DISPOSING
      6: t("taskState.disposed"), // DISPOSED
    };
    return stateMap[state] || `状态${state}`;
  };

  const getTypeColor = (type: number) => {
    const colors: Record<number, string> = {
      0: "blue", // TASK
      1: "green", // JOB
      2: "orange", // WORK
      3: "purple", // CHANNEL
    };
    return colors[type] || "default";
  };

  const getTypeText = (type: number) => {
    const typeMap: Record<number, string> = {
      0: t("taskType.task"), // TASK
      1: t("taskType.job"), // JOB
      2: t("taskType.work"), // WORK
      3: t("taskType.channel"), // CHANNEL
    };
    return typeMap[type] || `类型${type}`;
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
      // 对子任务按ID进行排序
      const sortedChildren = [...node.children].sort((a, b) => a.id - b.id);
      for (const child of sortedChildren) {
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

    if (days > 0) return `${days}${t("timeUnits.day")} ${hours % 24}${t("timeUnits.hour")}`;
    if (hours > 0) return `${hours}${t("timeUnits.hour")} ${minutes % 60}${t("timeUnits.minute")}`;
    if (minutes > 0) return `${minutes}${t("timeUnits.minute")} ${seconds % 60}${t("timeUnits.second")}`;
    return `${seconds}${t("timeUnits.second")}`;
  };

  const columns: ColumnsType<TaskInfo> = [
    {
      title: t("table.owner"),
      dataIndex: "ownerType",
      key: "ownerType",
      width: 200,
      render: (owner: string, record) => {
        const indent = "  ".repeat(record.level);
        return indent + owner;
      },
    },
    {
      title: t("table.taskId"),
      dataIndex: "id",
      key: "id",
      width: 80,
      sorter: (a, b) => a.id - b.id,
    },
    {
      title: t("table.type"),
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type: number) => (
        <Tag color={getTypeColor(type)}>{getTypeText(type)}</Tag>
      ),
      filters: [
        { text: t("taskType.task"), value: 0 },
        { text: t("taskType.job"), value: 1 },
        { text: t("taskType.work"), value: 2 },
        { text: t("taskType.channel"), value: 3 },
      ],
      onFilter: (value, record) => record.type === value,
    },
    {
      title: t("table.state"),
      dataIndex: "state",
      key: "state",
      width: 100,
      render: (state: number) => (
        <Tag color={getStateColor(state)}>{getStateText(state)}</Tag>
      ),
      filters: [
        { text: t("taskState.init"), value: 0 },
        { text: t("taskState.starting"), value: 1 },
        { text: t("taskState.started"), value: 2 },
        { text: t("taskState.running"), value: 3 },
        { text: t("taskState.going"), value: 4 },
        { text: t("taskState.disposing"), value: 5 },
        { text: t("taskState.disposed"), value: 6 },
      ],
      onFilter: (value, record) => record.state === value,
    },
    {
      title: t("table.startTime"),
      dataIndex: "startTime",
      key: "startTime",
      width: 180,
      render: (time: string) => new Date(time).toLocaleString(),
      sorter: (a, b) =>
        new Date(a.startTime).getTime() - new Date(b.startTime).getTime(),
    },
    {
      title: t("table.duration"),
      key: "duration",
      width: 120,
      render: (_, record) => formatDuration(record.startTime),
    },
    {
      title: t("table.retryCount"),
      dataIndex: "retryCount",
      key: "retryCount",
      width: 100,
      render: (retryCount: number, record) =>
        `${retryCount} / ${record.maxRetry === -1 ? "∞" : record.maxRetry}`,
      sorter: (a, b) => a.retryCount - b.retryCount,
    },
    {
      title: t("table.startReason"),
      dataIndex: "startReason",
      key: "startReason",
      width: 200,
      ellipsis: true,
    },
    {
      title: t("table.actions"),
      key: "actions",
      width: 120,
      fixed: "right",
      render: (_, record) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<ReloadOutlined />}
            onClick={() => handleRestartTask(record.id)}
            disabled={record.state === 6} // DISPOSED状态禁用
          >
            {t("task.restart")}
          </Button>
          <Button
            type="link"
            size="small"
            danger
            icon={<PauseCircleOutlined />}
            onClick={() => handleStopTask(record.id)}
            disabled={record.state === 6} // DISPOSED状态禁用
          >
            {t("task.stop")}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title={
        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
          <Logo size="small" showText={false} />
          <span>{t("task.tree")}</span>
        </div>
      }
      style={{ width: "100%", maxWidth: "none" }}
      extra={
        <Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreateDemoTask}
          >
            {t("task.createDemo")}
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={fetchTaskTree}
            loading={loading}
          >
            {t("task.refresh")}
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
          scroll={{ x: 1450 }}
          pagination={{
            pageSize: 50,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => t("task.total", { count: total }),
          }}
          size="small"
        />
      ) : (
        <div style={{ textAlign: "center", padding: "50px" }}>
          {t("task.noData")}
        </div>
      )}
    </Card>
  );
};

export default TaskTreeComponent;
