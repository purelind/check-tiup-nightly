'use client';

import { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'next/navigation';
import { CheckResult, ComponentsInfo } from '@/types';
import Link from 'next/link';

export default function PlatformHistory() {
  const params = useParams();
  const searchParams = useSearchParams();
  const [results, setResults] = useState<CheckResult[]>([]);
  const [loading, setLoading] = useState(true);
  
  const platform = params.platform as string;
  const days = searchParams.get('days') || '1';

  useEffect(() => {
    const fetchHistory = async () => {
      setLoading(true);
      try {
        // 修改为实际的后端 API 地址
        const response = await fetch(`/api/history/${platform}?days=${days}`)
        if (!response.ok) {
          throw new Error('Failed to fetch history');
        }
        const data = await response.json();
        setResults(data.results || []);
      } catch (error) {
        console.error('Failed to fetch history:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchHistory();
  }, [platform, days]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-4 border-primary border-t-transparent"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-8">
      <div className="max-w-7xl mx-auto px-4">
        <div className="mb-8">
          <Link 
            href="/"
            className="text-blue-600 hover:text-blue-800 mb-4 inline-block"
          >
            ← 返回首页
          </Link>
          <h1 className="text-3xl font-bold text-gray-900 mt-4">
            {decodeURIComponent(platform)} 检查历史
          </h1>
          
          {/* 时间范围选择器 */}
          <div className="mt-4 flex gap-4">
            {[
              { label: '最近一天', value: '1' },
              { label: '最近三天', value: '3' },
              { label: '最近七天', value: '7' },
            ].map(({ label, value }) => (
              <Link
                key={value}
                href={`/history/${platform}?days=${value}`}
                className={`px-4 py-2 rounded-full ${
                  days === value 
                    ? 'bg-blue-600 text-white' 
                    : 'bg-white text-gray-600 hover:bg-gray-100'
                }`}
              >
                {label}
              </Link>
            ))}
          </div>
        </div>

        {results.length === 0 ? (
          <div className="bg-white rounded-lg shadow p-6 text-center text-gray-500">
            暂无检查记录
          </div>
        ) : (
          <div className="space-y-4">
            {results.map((result, index) => (
              <div key={index} className="bg-white rounded-lg shadow p-6">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center">
                    <span className={`w-3 h-3 rounded-full mr-2 ${
                      result.status === 'success' ? 'bg-green-500' : 'bg-red-500'
                    }`}></span>
                    <span className={`text-sm font-medium ${
                      result.status === 'success' ? 'text-green-800' : 'text-red-800'
                    }`}>
                      {result.status === 'success' ? '正常' : '异常'}
                    </span>
                  </div>
                  <span className="text-sm text-gray-600">
                    {new Date(result.timestamp).toLocaleString()}
                  </span>
                </div>
                
                <div className="space-y-2 text-sm text-gray-600">
                  <p>TiUP 版本: {result.tiup_version}</p>
                  <p>Python 版本: {result.python_version}</p>
                    {/* 添加组件信息展示 */}
                    {result.components_info && (
                        <div className="mt-4">
                        <p className="font-medium text-gray-700 mb-2">组件信息:</p>
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            {(() => {
                            let componentsData: ComponentsInfo | null = null;
                            try {
                                componentsData = result.components_info ? JSON.parse(result.components_info) : null;
                            } catch (e) {
                                console.error('Failed to parse components info:', e);
                                return null;
                            }

                            if (!componentsData) return null;

                            return ['tidb', 'pd', 'tikv', 'tiflash'].map((component) => {
                                const info = componentsData[component as keyof ComponentsInfo];
                                return info ? (
                                <div key={component} className="bg-gray-50 rounded-lg p-3">
                                    <p className="font-medium text-gray-800 mb-1 capitalize">{component}</p>
                                    <div className="text-xs space-y-1">
                                    <p className="truncate" title={info.full_version}>
                                        版本: {info.full_version}
                                    </p>
                                    <p className="truncate" title={info.git_hash}>
                                        Git: {info.git_hash.substring(0, 8)}
                                    </p>
                                    <p className="truncate" title={info.base_version}>
                                        基础版本: {info.base_version}
                                    </p>
                                    </div>
                                </div>
                                ) : null;
                            });
                            })()}
                        </div>
                        </div>
                    )}

                    {/* 当没有组件信息时显示提示 */}
                    {!result.components_info && result.status === 'success' && (
                        <div className="mt-4">
                        <p className="text-sm text-gray-500">暂无组件信息</p>
                        </div>
                    )}
                  {result.errors && (
                    <div className="mt-4">
                      <p className="text-red-600 font-medium mb-2">错误信息:</p>
                      {Array.isArray(result.errors) ? result.errors.map((error, idx) => (
                        <div key={idx} className="mb-2 bg-red-50 rounded-lg p-4 border border-red-100">
                          {error.error && (
                            <div className="mb-2">
                              <span className="font-medium">错误原因：</span>
                              <span className="text-red-700">{error.error}</span>
                            </div>
                          )}
                          {error.stage && (
                            <div className="mb-2">
                              <span className="font-medium">错误阶段：</span>
                              <span className="text-gray-700">{error.stage}</span>
                            </div>
                          )}
                          {error.timestamp && (
                            <div>
                              <span className="font-medium">发生时间：</span>
                              <span className="text-gray-700">
                                {new Date(error.timestamp).toLocaleString()}
                              </span>
                            </div>
                          )}
                        </div>
                      )) : (
                        <div className="bg-red-50 rounded-lg p-4 border border-red-100">
                          <pre className="text-sm text-red-700 whitespace-pre-wrap break-words">
                            {JSON.stringify(result.errors, null, 2)}
                          </pre>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
