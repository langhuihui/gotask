import axios from 'axios';
import type { TaskInfo, TaskTree, TaskHistory, TaskStats } from '../types/task';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080/api';

const api = axios.create({
  baseURL: API_BASE,
  timeout: 5000,
});

export const taskApi = {
  // 获取任务树
  getTaskTree: async (): Promise<TaskTree> => {
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

  // 获取任务历史
  getTaskHistory: async (): Promise<TaskHistory[]> => {
    const response = await api.get('/tasks/history');
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