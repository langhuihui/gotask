import axios from 'axios';
import type { TaskInfo, TaskStats, TaskHistoryResponse, SessionInfo, TaskHistoryFilter } from '../types/task';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8082/api';

const api = axios.create({
  baseURL: API_BASE,
  timeout: 5000,
});

export const taskApi = {
  // 获取任务树
  getTaskTree: async (): Promise<TaskInfo> => {
    const response = await api.get('/tasks/tree');
    return response.data;
  },

  // 获取所有任务列表
  getTasks: async (): Promise<TaskInfo[]> => {
    const response = await api.get('/tasks');
    return response.data;
  },

  // 获取任务详情
  getTask: async (id: number): Promise<TaskInfo> => {
    const response = await api.get(`/tasks/${id}`);
    return response.data;
  },

  // 停止任务
  stopTask: async (id: number, reason?: string): Promise<void> => {
    await api.post(`/tasks/${id}/stop`, { reason });
  },

  // 获取任务历史（支持过滤和分页）
  getTaskHistory: async (filter?: TaskHistoryFilter): Promise<TaskHistoryResponse> => {
    const params = new URLSearchParams();
    if (filter) {
      if (filter.ownerType) params.append('ownerType', filter.ownerType);
      if (filter.taskType !== undefined) params.append('taskType', filter.taskType.toString());
      if (filter.sessionId) params.append('sessionId', filter.sessionId);
      if (filter.parentId) params.append('parentId', filter.parentId.toString());
      if (filter.startTime) params.append('startTime', filter.startTime);
      if (filter.endTime) params.append('endTime', filter.endTime);
      if (filter.limit) params.append('limit', filter.limit.toString());
      if (filter.offset) params.append('offset', filter.offset.toString());
    }
    const response = await api.get(`/tasks/history?${params.toString()}`);
    return response.data;
  },

  // 获取任务历史统计
  getTaskHistoryStats: async (): Promise<any> => {
    const response = await api.get('/tasks/history/stats');
    return response.data;
  },

  // 获取会话信息
  getSessionInfo: async (): Promise<SessionInfo> => {
    const response = await api.get('/session');
    return response.data;
  },

  // 获取任务统计
  getTaskStats: async (): Promise<TaskStats> => {
    const response = await api.get('/tasks/stats');
    return response.data;
  },

  // 创建示例任务
  createDemoTask: async (): Promise<void> => {
    await api.post('/tasks');
  },
};