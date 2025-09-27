import React from "react";
import { Card, Descriptions, Tag, Button, Space, Divider } from "antd";
import { DeleteOutlined, ReloadOutlined } from "@ant-design/icons";
import type { TaskInfo } from "../types/task";
import { useLanguage } from "../hooks/useLanguage";

interface TaskDetailProps {
  task: TaskInfo | null;
  onStopTask?: (taskId: number) => void;
  onRefresh?: () => void;
}

const TaskDetail: React.FC<TaskDetailProps> = ({
  task,
  onStopTask,
  onRefresh,
}) => {
  const { t } = useLanguage();

  if (!task) {
    return (
      <Card title={t("task.detail")}>
        <div style={{ textAlign: "center", padding: "50px" }}>
          {t("task.selectTask")}
        </div>
      </Card>
    );
  }

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

  const getTypeColor = (type: number) => {
    const colors: Record<number, string> = {
      0: "blue", // TASK
      1: "green", // JOB
      2: "orange", // WORK
      3: "purple", // CHANNEL
    };
    return colors[type] || "default";
  };

  const getStateText = (state: number) => {
    const stateMap: Record<number, string> = {
      0: t("taskState.init"),
      1: t("taskState.starting"),
      2: t("taskState.started"),
      3: t("taskState.running"),
      4: t("taskState.going"),
      5: t("taskState.disposing"),
      6: t("taskState.disposed"),
    };
    return stateMap[state] || `状态${state}`;
  };

  const getTypeText = (type: number) => {
    const typeMap: Record<number, string> = {
      0: t("taskType.task"),
      1: t("taskType.job"),
      2: t("taskType.work"),
      3: t("taskType.channel"),
    };
    return typeMap[type] || `类型${type}`;
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

  return (
    <Card
      title={t("task.detail")}
      extra={
        <Space>
          {task.state !== 6 && (
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={() => onStopTask?.(task.id)}
            >
              {t("task.stopTask")}
            </Button>
          )}
          <Button icon={<ReloadOutlined />} onClick={onRefresh}>
            {t("task.refresh")}
          </Button>
        </Space>
      }
    >
      <Descriptions column={2} bordered>
        <Descriptions.Item label={t("taskDetail.taskId")}>
          {task.id}
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.taskType")}>
          <Tag color={getTypeColor(task.type)}>{getTypeText(task.type)}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.ownerType")}>
          {task.owner}
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.taskState")}>
          <Tag color={getStateColor(task.state)}>
            {getStateText(task.state)}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.taskLevel")}>
          {task.level}
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.retryCount")}>
          {task.retryCount} / {task.maxRetry === -1 ? "∞" : task.maxRetry}
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.startTime")}>
          {new Date(task.startTime).toLocaleString()}
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.duration")}>
          {formatDuration(task.startTime)}
        </Descriptions.Item>
        <Descriptions.Item label={t("taskDetail.startReason")} span={2}>
          {task.startReason}
        </Descriptions.Item>
        {task.stopReason && (
          <Descriptions.Item label={t("taskDetail.stopReason")} span={2}>
            <Tag color="error">{task.stopReason}</Tag>
          </Descriptions.Item>
        )}
        {task.parentId && (
          <Descriptions.Item label={t("taskDetail.parentTaskId")}>
            {task.parentId}
          </Descriptions.Item>
        )}
        {task.children && task.children.length > 0 && (
          <Descriptions.Item label={t("taskDetail.childTaskCount")}>
            {task.children.length}
          </Descriptions.Item>
        )}
      </Descriptions>

      {Object.keys(task.description).length > 0 && (
        <>
          <Divider />
          <Descriptions title={t("taskDetail.description")} column={2} bordered>
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
