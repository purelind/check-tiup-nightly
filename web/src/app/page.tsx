'use client';  // Because we used useState, it needs to be marked as a client component.

import { useEffect, useState } from 'react';
import { CheckResult } from '../types';
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
              {/* Modify the title to be a clickable link */}
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
                {result.status === 'success' ? 'Normal' : 'Abnormal'}
              </span>
            </div>
              
              <div className="space-y-2 text-sm text-gray-600">
                <p>Check Time: {new Date(result.timestamp).toLocaleString()}</p>
                <p>TiUP Version: {
                  typeof result.version.tiup === 'string' 
                    ? result.version.tiup.split(' ')[0].replace(/^v/, '')
                    : 'Unknown'
                }</p>
                
                {/* component information */}
                {result.version.components && Object.keys(result.version.components).length > 0 && (
                  <div className="mt-4">
                    <p className="font-medium text-gray-700 mb-2">Component Information:</p>
                    <div className="grid grid-cols-2 gap-4">
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
      </div>
    </div>
  );
}
