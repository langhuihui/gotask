import React, { useState, useEffect } from "react";
import {
  Row,
  Col,
  Card,
  Table,
  Radio,
  Button,
  Space,
  Select,
  DatePicker,
  Input,
  Form,
  Statistic,
  Tag,
  Tooltip,
  Empty,
  message,
} from "antd";
import {
  ReloadOutlined,
  FilterOutlined,
  BarChartOutlined,
  TableOutlined,
  ClockCircleOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type {
  TaskHistory,
  SessionInfo,
  TaskHistoryFilter,
} from "../types/task";
import { taskApi } from "../services/api";
import { useLanguage } from "../hooks/useLanguage";
import dayjs from "dayjs";

const { RangePicker } = DatePicker;
const { Option } = Select;

interface TaskHistoryViewProps {
  onTaskSelect?: (taskId: number) => void;
}

const TaskHistoryView: React.FC<TaskHistoryViewProps> = ({ onTaskSelect }) => {
  const { t } = useLanguage();
  const [form] = Form.useForm();

  // State
  const [sessionList, setSessionList] = useState<SessionInfo[]>([]);
  const [selectedSession, setSelectedSession] = useState<SessionInfo | null>(
    null
  );
  const [taskList, setTaskList] = useState<TaskHistory[]>([]);
  const [currentView, setCurrentView] = useState<"table" | "gantt">("table");
  const [loadingSession, setLoadingSession] = useState(false);
  const [loadingTask, setLoadingTask] = useState(false);
  const [historyStats, setHistoryStats] = useState<any>(null);
  const [filter, setFilter] = useState<TaskHistoryFilter>({});

  // 获取会话列表
  const fetchSessionList = async () => {
    setLoadingSession(true);
    try {
      const sessionInfo = await taskApi.getSessionInfo();
      setSessionList([sessionInfo]);
      setSelectedSession(sessionInfo);
      await fetchTaskList(sessionInfo.id);
    } catch (error) {
      message.error("获取会话信息失败");
    } finally {
      setLoadingSession(false);
    }
  };

  // 获取任务历史
  const fetchTaskList = async (sessionId?: string) => {
    setLoadingTask(true);
    try {
      const currentFilter = { ...filter };
      if (sessionId) {
        currentFilter.sessionId = sessionId;
      }
      const response = await taskApi.getTaskHistory(currentFilter);
      setTaskList(response.tasks);
    } catch (error) {
      message.error("获取任务历史失败");
    } finally {
      setLoadingTask(false);
    }
  };

  // 获取历史统计
  const fetchHistoryStats = async () => {
    try {
      const stats = await taskApi.getTaskHistoryStats();
      setHistoryStats(stats);
    } catch (error) {
      console.error("获取历史统计失败:", error);
    }
  };

  // 应用过滤器
  const applyFilter = async () => {
    const values = await form.validateFields();
    const newFilter: TaskHistoryFilter = {
      ...values,
      startTime: values.timeRange?.[0]?.format("YYYY-MM-DDTHH:mm:ssZ"),
      endTime: values.timeRange?.[1]?.format("YYYY-MM-DDTHH:mm:ssZ"),
    };
    // 删除timeRange属性，因为它不是TaskHistoryFilter的一部分
    const { timeRange, ...filterWithoutTimeRange } = newFilter as any;
    setFilter(filterWithoutTimeRange);
    await fetchTaskList(selectedSession?.id);
  };

  // 重置过滤器
  const resetFilter = () => {
    form.resetFields();
    setFilter({});
    fetchTaskList(selectedSession?.id);
  };

  useEffect(() => {
    fetchSessionList();
    fetchHistoryStats();
  }, []);

  // 格式化时间
  const formatDateTimeShort = (date: string) => {
    if (!date) return "-";
    const dateObj = dayjs(date);
    const now = dayjs();

    if (!dateObj.isValid()) return "-";

    // 如果是同一天，只显示时间
    if (dateObj.isSame(now, "day")) {
      return dateObj.format("HH:mm:ss");
    }
    // 如果是同一年，不显示年份
    if (dateObj.isSame(now, "year")) {
      return dateObj.format("MM-DD HH:mm");
    }
    // 否则显示完整日期
    return dateObj.format("YYYY-MM-DD HH:mm");
  };

  // 获取类型颜色
  const getTypeColor = (type: number) => {
    const colors: Record<number, string> = {
      0: "blue", // TASK
      1: "green", // JOB
      2: "orange", // WORK
      3: "purple", // CHANNEL
    };
    return colors[type] || "default";
  };

  // 格式化持续时间
  const formatDuration = (duration: number) => {
    const seconds = Math.floor(duration / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}${t("timeUnits.day")} ${hours % 24}${t("timeUnits.hour")}`;
    if (hours > 0) return `${hours}${t("timeUnits.hour")} ${minutes % 60}${t("timeUnits.minute")}`;
    if (minutes > 0) return `${minutes}${t("timeUnits.minute")} ${seconds % 60}${t("timeUnits.second")}`;
    return `${seconds}${t("timeUnits.second")}`;
  };

  // 会话列表列定义
  const sessionColumns: ColumnsType<SessionInfo> = [
    {
      title: t("session.id"),
      dataIndex: "id",
      key: "id",
      width: 120,
    },
    {
      title: t("session.startTime"),
      dataIndex: "startTime",
      key: "startTime",
      width: 150,
      render: (time: string) => formatDateTimeShort(time),
    },
    {
      title: t("session.endTime"),
      dataIndex: "endTime",
      key: "endTime",
      width: 150,
      render: (time: string) => formatDateTimeShort(time),
    },
  ];

  // 任务列表列定义
  const taskColumns: ColumnsType<TaskHistory> = [
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
      width: 80,
      render: (type: number) => {
        const typeNames = ["TASK", "JOB", "WORK", "CHANNEL"];
        return (
          <Tag color={getTypeColor(type)}>{typeNames[type] || "UNKNOWN"}</Tag>
        );
      },
    },
    {
      title: t("table.owner"),
      dataIndex: "ownerType",
      key: "ownerType",
      width: 120,
    },
    {
      title: t("table.startTime"),
      dataIndex: "startTime",
      key: "startTime",
      width: 150,
      render: (time: string) => formatDateTimeShort(time),
      sorter: (a, b) =>
        new Date(a.startTime).getTime() - new Date(b.startTime).getTime(),
    },
    {
      title: t("table.endTime"),
      dataIndex: "endTime",
      key: "endTime",
      width: 150,
      render: (time: string) => formatDateTimeShort(time),
    },
    {
      title: t("table.duration"),
      dataIndex: "duration",
      key: "duration",
      width: 100,
      render: (duration: number) => formatDuration(duration),
      sorter: (a, b) => a.duration - b.duration,
    },
    {
      title: t("table.retryCount"),
      dataIndex: "retryCount",
      key: "retryCount",
      width: 80,
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
  ];

  // 会话行选择
  const handleSessionSelect = (session: SessionInfo) => {
    setSelectedSession(session);
    fetchTaskList(session.id);
  };

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column" }}>
      <Row gutter={16} style={{ flex: 1, minHeight: 0 }}>
        {/* 左侧：会话列表 */}
        <Col span={6}>
          <Card
            title={t("history.sessionList")}
            variant="outlined"
            style={{ height: "100%", display: "flex", flexDirection: "column" }}
            styles={{ body: { flex: 1, overflow: "hidden" } }}
          >
            <Table
              columns={sessionColumns}
              dataSource={sessionList}
              loading={loadingSession}
              pagination={false}
              scroll={{ y: "calc(100vh - 200px)" }}
              rowKey="id"
              size="small"
              onRow={(record) => ({
                onClick: () => handleSessionSelect(record),
                style: {
                  cursor: "pointer",
                  backgroundColor:
                    selectedSession?.id === record.id ? "#e6f7ff" : undefined,
                },
              })}
            />
          </Card>
        </Col>

        {/* 右侧：任务详情 */}
        <Col span={18}>
          <Card
            variant="outlined"
            style={{ height: "100%", display: "flex", flexDirection: "column" }}
            styles={{ body: { flex: 1, overflow: "hidden" } }}
            extra={
              <Space>
                <Radio.Group
                  value={currentView}
                  onChange={(e) => setCurrentView(e.target.value)}
                  buttonStyle="solid"
                >
                  <Radio.Button value="table">
                    <TableOutlined /> {t("history.tableView")}
                  </Radio.Button>
                  <Radio.Button value="gantt">
                    <ClockCircleOutlined /> {t("history.ganttView")}
                  </Radio.Button>
                </Radio.Group>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={() => fetchTaskList(selectedSession?.id)}
                  loading={loadingTask}
                >
                  {t("history.refresh")}
                </Button>
              </Space>
            }
          >
            {/* 过滤器 */}
            <Card size="small" style={{ marginBottom: 16 }}>
              <Form form={form} layout="inline" onFinish={applyFilter}>
                <Form.Item name="ownerType" label={t("history.ownerType")}>
                  <Input
                    placeholder={t("history.ownerType")}
                    style={{ width: 120 }}
                  />
                </Form.Item>
                <Form.Item name="taskType" label={t("history.taskType")}>
                  <Select
                    placeholder={t("history.taskType")}
                    style={{ width: 120 }}
                    allowClear
                  >
                    <Option value={0}>TASK</Option>
                    <Option value={1}>JOB</Option>
                    <Option value={2}>WORK</Option>
                    <Option value={3}>CHANNEL</Option>
                  </Select>
                </Form.Item>
                <Form.Item name="timeRange" label={t("history.timeRange")}>
                  <RangePicker showTime />
                </Form.Item>
                <Form.Item>
                  <Space>
                    <Button
                      type="primary"
                      htmlType="submit"
                      icon={<FilterOutlined />}
                    >
                      {t("history.apply")}
                    </Button>
                    <Button onClick={resetFilter}>{t("history.reset")}</Button>
                  </Space>
                </Form.Item>
              </Form>
            </Card>

            {/* 统计信息 */}
            {historyStats && (
              <Card size="small" style={{ marginBottom: 16 }}>
                <Row gutter={16}>
                  <Col span={6}>
                    <Statistic
                      title={t("history.totalTasks")}
                      value={historyStats.totalTasks}
                      prefix={<BarChartOutlined />}
                    />
                  </Col>
                  <Col span={6}>
                    <Statistic
                      title={t("history.totalDuration")}
                      value={historyStats.totalDuration}
                    />
                  </Col>
                  <Col span={6}>
                    <Statistic
                      title={t("history.averageDuration")}
                      value={historyStats.averageDuration}
                    />
                  </Col>
                  <Col span={6}>
                    <Statistic
                      title="会话ID"
                      value={historyStats.sessionId}
                      valueStyle={{ fontSize: 12 }}
                    />
                  </Col>
                </Row>
              </Card>
            )}

            {/* 表格视图 */}
            {currentView === "table" && (
              <Table
                columns={taskColumns}
                dataSource={taskList}
                loading={loadingTask}
                pagination={{
                  total: taskList?.length || 0,
                  pageSize: 20,
                  showSizeChanger: true,
                  showQuickJumper: true,
                  showTotal: (total) => t("history.total", { count: total }),
                }}
                scroll={{ y: "calc(100vh - 400px)" }}
                size="small"
                rowKey="id"
                onRow={(record) => ({
                  onClick: () => onTaskSelect?.(record.id),
                  style: { cursor: "pointer" },
                })}
              />
            )}

            {/* 甘特图视图 */}
            {currentView === "gantt" && (
              <div style={{ height: "calc(100vh - 400px)", overflow: "auto" }}>
                {taskList.length > 0 ? (
                  <div>
                    <p>甘特图视图功能需要安装额外的图表库</p>
                    <p>当前显示 {taskList.length} 个任务</p>
                    {/* 这里可以集成甘特图组件 */}
                  </div>
                ) : (
                  <Empty description="暂无任务数据" />
                )}
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default TaskHistoryView;
