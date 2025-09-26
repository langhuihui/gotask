import { useState, useEffect, useCallback } from 'react';
import zh from '../locales/zh.json';
import en from '../locales/en.json';

type Language = 'zh' | 'en';

const getStoredLanguage = (): Language => {
  const stored = localStorage.getItem('i18nextLng');
  return (stored as Language) || 'zh';
};

const getBrowserLanguage = (): Language => {
  const lang = navigator.language.toLowerCase();
  if (lang.startsWith('zh')) return 'zh';
  return 'en';
};

// 全局状态管理
let globalLanguage: Language = getStoredLanguage() || getBrowserLanguage();
let globalTranslations = globalLanguage === 'zh' ? zh : en;
const listeners = new Set<() => void>();

const notifyListeners = () => {
  listeners.forEach(listener => listener());
};

export const useLanguage = () => {
  const [, forceUpdate] = useState({});

  const rerender = useCallback(() => {
    forceUpdate({});
  }, []);

  useEffect(() => {
    listeners.add(rerender);
    return () => {
      listeners.delete(rerender);
    };
  }, [rerender]);

  const t = useCallback((key: string, options?: { [key: string]: any; }): string => {
    const keys = key.split('.');
    let value: any = globalTranslations;

    for (const k of keys) {
      value = value?.[k];
    }

    if (typeof value !== 'string') {
      return key;
    }

    if (options) {
      return value.replace(/\{\{(\w+)\}\}/g, (match, key) => {
        return options[key] || match;
      });
    }

    return value;
  }, []);

  const changeLanguage = useCallback((lng: Language) => {
    globalLanguage = lng;
    globalTranslations = lng === 'zh' ? zh : en;
    localStorage.setItem('i18nextLng', lng);
    notifyListeners();
  }, []);

  const getCurrentLanguage = useCallback(() => {
    return globalLanguage;
  }, []);

  const isChinese = useCallback(() => {
    return globalLanguage === 'zh';
  }, []);

  const isEnglish = useCallback(() => {
    return globalLanguage === 'en';
  }, []);

  return {
    t,
    changeLanguage,
    getCurrentLanguage,
    isChinese,
    isEnglish,
    currentLanguage: globalLanguage
  };
};
