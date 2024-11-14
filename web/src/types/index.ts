export interface ComponentInfo {
  full_version: string;
  base_version: string;
  git_hash: string;
}

export interface ComponentsInfo {
  tidb?: ComponentInfo;
  pd?: ComponentInfo;
  tikv?: ComponentInfo;
  tiflash?: ComponentInfo;
}

export interface ErrorDetail {
  stage: string;
  error: string;
  timestamp: string;
}

export interface CheckResult {
  id: number;
  platform: string;
  status: 'success' | 'failed';
  timestamp: string;
  tiup_version: string;
  python_version: string;
  os: string;
  arch: string;
  errors?: ErrorDetail[];
  components_info?: string | null;
}