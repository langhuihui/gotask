import React, { useState, useEffect } from "react";
import { Card, Table, Tag, Button, Space, Tooltip, DatePicker } from "antd";
import { ReloadOutlined, EyeOutlined } from "@ant-design/icons";
import type { TaskHistory } from "../types/task";
import { taskApi } from "../services/api";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import { useLanguage } from "../hooks/useLanguage";

const { RangePicker } = DatePicker;

interface TaskHistoryProps {
  onTaskSelect?: (taskId: number) => void;
}

const TaskHistoryComponent: React.FC<TaskHistoryProps> = ({ onTaskSelect }) => {
  const { t } = useLanguage();
  const [history, setHistory] = useState<TaskHistory[]>([]);
  const [loading, setLoading] = useState(false);
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(
    null
  );

  const fetchHistory = async () => {
    setLoading(true);
    try {
      const data = await taskApi.getTaskHistory();

      // 如果有日期范围，则过滤数据
      let filteredData = data;
      if (dateRange) {
        const [start, end] = dateRange;
        filteredData = data.filter((item) => {
          const itemDate = dayjs(item.startTime);
          return (
            itemDate.isAfter(start.startOf("day")) &&
            itemDate.isBefore(end.endOf("day"))
          );
        });
      }

      setHistory(filteredData);
    } catch (error) {
      console.error("获取任务历史失败:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHistory();
    const interval = setInterval(fetchHistory, 5000);
    return () => clearInterval(interval);
  }, [dateRange]);

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

  const getTypeColor = (type: string) => {
    const colors: Record<string, string> = {
      TASK: "blue",
      JOB: "green",
      WORK: "orange",
      CHANNEL: "purple",
    };
    return colors[type] || "default";
  };

  const formatDuration = (duration: number) => {
    const seconds = Math.floor(duration / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
    return `${seconds}s`;
  };

  const columns: ColumnsType<TaskHistory> = [
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
      render: (type: string) => <Tag color={getTypeColor(type)}>{type}</Tag>,
      filters: [
        { text: "TASK", value: "TASK" },
        { text: "JOB", value: "JOB" },
        { text: "WORK", value: "WORK" },
        { text: "CHANNEL", value: "CHANNEL" },
      ],
      onFilter: (value, record) => record.type === value,
    },
    {
      title: t("table.owner"),
      dataIndex: "ownerType",
      key: "ownerType",
      width: 120,
    },
    {
      title: t("table.state"),
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
      title: t("table.startTime"),
      dataIndex: "startTime",
      key: "startTime",
      width: 180,
      render: (time: string) => new Date(time).toLocaleString(),
      sorter: (a, b) =>
        new Date(a.startTime).getTime() - new Date(b.startTime).getTime(),
    },
    {
      title: t("table.endTime"),
      dataIndex: "endTime",
      key: "endTime",
      width: 180,
      render: (time: string) => new Date(time).toLocaleString(),
    },
    {
      title: t("table.duration"),
      dataIndex: "duration",
      key: "duration",
      width: 120,
      render: (duration: number) => formatDuration(duration),
      sorter: (a, b) => a.duration - b.duration,
    },
    {
      title: t("table.retryCount"),
      dataIndex: "retryCount",
      key: "retryCount",
      width: 100,
      sorter: (a, b) => a.retryCount - b.retryCount,
    },
    {
      title: t("table.stopReason"),
      dataIndex: "stopReason",
      key: "stopReason",
      width: 150,
      render: (reason: string) =>
        reason ? (
          <Tooltip title={reason}>
            <Tag color="error">
              {reason.length > 20 ? reason.substring(0, 20) + "..." : reason}
            </Tag>
          </Tooltip>
        ) : (
          "-"
        ),
    },
    {
      title: t("table.actions"),
      key: "action",
      width: 80,
      render: (_, record) => (
        <Button
          type="link"
          icon={<EyeOutlined />}
          onClick={() => onTaskSelect?.(record.id)}
        >
          {t("history.view")}
        </Button>
      ),
    },
  ];

  return (
    <Card
      title={t("history.title")}
      extra={
        <Space>
          <RangePicker
            onChange={(dates) => setDateRange(dates as any)}
            placeholder={[t("history.startDate"), t("history.endDate")]}
          />
          <Button
            icon={<ReloadOutlined />}
            onClick={fetchHistory}
            loading={loading}
          >
            {t("history.refresh")}
          </Button>
        </Space>
      }
    >
      <Table
        columns={columns}
        dataSource={history}
        loading={loading}
        rowKey="id"
        scroll={{ x: 1200 }}
        pagination={{
          total: history.length,
          pageSize: 20,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t("history.total", { count: total }),
        }}
      />
    </Card>
  );
};

export default TaskHistoryComponent;
