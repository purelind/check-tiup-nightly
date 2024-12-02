export interface ComponentInfo {
  full_version: string;
  base_version: string;
  git_hash: string;
  commit_time: string;
}

export interface ErrorDetail {
  stage: string;
  error: string;
  timestamp: string;
}

export interface VersionInfo {
  tiup: string;
  python?: string;
  components?: Record<string, ComponentInfo>;
}

export interface CheckResult {
  id: number;
  platform: string;
  status: 'success' | 'failed';
  timestamp: string;
  os: string;
  arch: string;
  errors?: ErrorDetail[];
  version: VersionInfo;
}