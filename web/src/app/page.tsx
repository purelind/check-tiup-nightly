'use client';  // 因为我们使用了 useState，需要标记为客户端组件

import { useEffect, useState } from 'react';
import { CheckResult, ComponentsInfo, ComponentInfo } from '../types';
import Link from 'next/link';

export default function HomePage() {
  const [results, setResults] = useState<CheckResult[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchResults = async () => {
      try {
        const response = await fetch('/api/results');
        const data = await response.json();
        console.log(data);
        setResults(data);
      } catch (error) {
        console.error('Failed to fetch results:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchResults();
  }, []);

  const allSuccessful = results.every(r => r.status === 'success');

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
        {/* <div className="mb-8 text-center">
          <h1 className="text-3xl font-bold text-gray-900">TiUP Nightly Status</h1>
          <div className={`mt-4 inline-flex items-center px-4 py-2 rounded-full ${
            allSuccessful ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
          }`}>
            <div className={`w-3 h-3 rounded-full mr-2 ${
              allSuccessful ? 'bg-green-500' : 'bg-red-500'
            }`}></div>
            {allSuccessful ? '所有平台正常' : '存在异常平台'}
          </div>
        </div> */}
        <div className="mb-8 text-center">
        <h1 className="text-3xl font-bold text-gray-900">TiUP Nightly Status</h1>
        <div className={`mt-6 p-4 rounded-lg ${
          allSuccessful ? 'bg-green-500' : 'bg-red-500'
        }`}>
          <div className="flex items-center">
            <svg className="w-6 h-6 text-white mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              {allSuccessful ? (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              )}
            </svg>
            <span className="text-white text-lg font-medium">
              {allSuccessful ? 'All Platforms Available' : 'Some Platforms Not Available'}
            </span>
          </div>
        </div>
      </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {results.map((result) => (
            <div key={result.platform} className="bg-white rounded-lg shadow p-6">
              <div className="flex items-center justify-between mb-4">
              {/* 修改标题为可点击的链接 */}
              <Link 
                href={`/history/${encodeURIComponent(result.platform)}`}
                className="text-lg font-semibold text-gray-900 hover:text-blue-600 hover:underline"
              >
                {result.platform}
              </Link>
              <span className={`px-3 py-1 rounded-full text-sm ${
                result.status === 'success' 
                  ? 'bg-green-100 text-green-800' 
                  : 'bg-red-100 text-red-800'
              }`}>
                {result.status === 'success' ? '正常' : '异常'}
              </span>
            </div>
              
              <div className="space-y-2 text-sm text-gray-600">
                <p>检查时间: {new Date(result.timestamp).toLocaleString()}</p>
                <p>TiUP 版本: {result.tiup_version.split(' ')[0]}</p>
                
                {/* 添加组件信息展示 */}
              {result.components_info && (
                <div className="mt-4">
                  <p className="font-medium text-gray-700 mb-2">组件信息:</p>
                  <div className="grid grid-cols-2 gap-4">
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
                    {Array.isArray(result.errors) ? result.errors.map((error, index) => (
                      <div key={index} className="mb-2 bg-red-50 rounded-lg p-4 border border-red-100">
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
      </div>
    </div>
  );
}
