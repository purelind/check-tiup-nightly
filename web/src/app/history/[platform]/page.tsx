'use client';

import { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'next/navigation';
import { CheckResult } from '@/types';
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
            ← Back to Home
          </Link>
          <h1 className="text-3xl font-bold text-gray-900 mt-4">
            {decodeURIComponent(platform)} Check History
          </h1>
          
          {/* Time Range Selector */}
          <div className="mt-4 flex gap-4">
            {[
              { label: 'Last Day', value: '1' },
              { label: 'Last 3 Days', value: '3' },
              { label: 'Last 7 Days', value: '7' },
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
            No check records found
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
                      {result.status === 'success' ? 'Normal' : 'Abnormal'}
                    </span>
                  </div>
                  <span className="text-sm text-gray-600">
                    {new Date(result.timestamp).toLocaleString()}
                  </span>
                </div>
                
                <div className="space-y-2 text-sm text-gray-600">
                  <p>TiUP Version: {
                    typeof result.version.tiup === 'string' 
                      ? result.version.tiup.split(' ')[0].replace(/^v/, '')
                      : 'Unknown'
                  }</p>
                  {/* Component Information Display */}
                  {result.version.components && Object.keys(result.version.components).length > 0 && (
                    <div className="mt-4">
                      <p className="font-medium text-gray-700 mb-2">Component Information:</p>
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        {['tidb', 'pd', 'tikv', 'tiflash'].map((component) => {
                          const info = result.version.components?.[component];
                          return info ? (
                            <div key={component} className="bg-gray-50 rounded-lg p-3">
                              <p className="font-medium text-gray-800 mb-1 capitalize">{component}</p>
                              <div className="text-xs space-y-1">
                              <p className="truncate" title={info.git_hash}>
                                Git: <a 
                                  href={`https://github.com/${
                                    component === 'pd' || component === 'tikv' 
                                      ? 'tikv' 
                                      : 'pingcap'
                                  }/${component}/commit/${info.git_hash}`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="text-blue-600 hover:underline"
                                >
                                  {info.git_hash.substring(0, 8)}
                                </a>
                              </p>
                                <p className="truncate" title={info.full_version}>
                                  Version: {info.full_version}
                                </p>
                                <p className="truncate" title={info.base_version}>
                                  Base Version: {info.base_version}
                                </p>
                              </div>
                            </div>
                          ) : null;
                        })}
                      </div>
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
