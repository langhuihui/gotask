export interface TaskInfo {
  id: number;
  type: number; // 0=TASK, 1=JOB, 2=WORK, 3=CHANNEL
  owner: string;
  startTime: string;
  description: Record<string, string>;
  state: number; // 0=INIT, 1=STARTING, 2=STARTED, 3=RUNNING, 4=GOING, 5=DISPOSING, 6=DISPOSED
  blocked?: TaskInfo;
  blocking?: boolean;
  pointer: string;
  children?: TaskInfo[];
  parent?: TaskInfo;
  parentId?: number;
  eventLoopRunning: boolean;
  level: number;
  startReason: string;
  stopReason?: string;
  retryCount: number;
  maxRetry: number;
}

export interface TaskHistory {
  id: number;
  type: 'TASK' | 'JOB' | 'WORK' | 'CHANNEL';
  ownerType: string;
  startTime: string;
  endTime: string;
  duration: number;
  state: 'INIT' | 'STARTING' | 'STARTED' | 'RUNNING' | 'GOING' | 'DISPOSING' | 'DISPOSED';
  stopReason?: string;
  retryCount: number;
  descriptions: Record<string, string>;
}

export interface TaskStats {
  totalTasks: number;
  runningTasks: number;
  completedTasks: number;
  failedTasks: number;
  retryCount: number;
}

export interface TaskTree {
  root: TaskInfo;
}

// 展平的任务列表（用于表格显示）
export interface FlatTaskInfo extends TaskInfo {
  key: number;
}