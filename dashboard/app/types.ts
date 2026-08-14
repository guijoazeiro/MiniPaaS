export type Theme = "light" | "dark";

export type User = {
  id: string;
  username: string;
  created_at: string;
};

export type App = {
  id: string;
  name: string;
  status: "idle" | "running" | "failed" | "stopped";
  container_state?: string;
  public_url?: string;
  created_at: string;
  updated_at: string;
};

export type CustomDomain = {
  id: string;
  app_id: string;
  hostname: string;
  status: "pending" | "verified" | "active" | "error";
  last_error?: string;
  verified_at?: string;
  created_at: string;
  updated_at: string;
};

export type AppMetrics = {
  app_name: string;
  collected_at: string;
  runtime?: {
    container_id?: string;
    state: string;
    restart_count: number;
    uptime_seconds: number;
    started_at?: string;
    cpu_percent: number;
    memory_usage_bytes: number;
    memory_limit_bytes: number;
    memory_percent: number;
    network_rx_bytes: number;
    network_tx_bytes: number;
    block_read_bytes: number;
    block_write_bytes: number;
    pids: number;
  };
  deployments: {
    total: number;
    successful: number;
    failed: number;
    in_progress: number;
    success_rate: number;
    average_duration_ms: number;
  };
  health_check_failures: Array<{
    deployment_id: string;
    message: string;
    created_at: string;
  }>;
};

export type MetricsPoint = {
  ts: string;
  cpu_percent: number;
  memory_percent: number;
  memory_usage_bytes: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  block_read_bytes: number;
  block_write_bytes: number;
};

export type LiveMetricsSnapshot = MetricsPoint & {
  state: string;
  restart_count: number;
  uptime_seconds: number;
  started_at?: string;
  memory_limit_bytes: number;
  pids: number;
  container_id?: string;
};

export type Deployment = {
  id: string;
  app_id: string;
  app_name?: string;
  image_tag: string;
  status: "pending" | "building" | "cancel_requested" | "cancelled" | "running" | "failed" | "superseded" | "rolled_back" | "stopped";
  port?: number;
  duration_ms?: number;
  created_at: string;
  source_type?: "upload" | "git";
  repository?: string;
  branch?: string;
  commit_sha?: string;
  commit_author?: string;
  commit_message?: string;
  trigger_type?: "manual" | "webhook";
  github_delivery_id?: string;
  attempt?: number;
  retry_of?: string;
  cancel_requested?: boolean;
};

export type DeploymentPage = {
  items: Deployment[];
  page: number;
  per_page: number;
  total: number;
};

export type DeploymentLog = {
  id: number;
  deployment_id: string;
  stage: string;
  stream: "stdout" | "stderr" | "system";
  message: string;
  created_at: string;
};

export type GitSource = {
  app_id: string;
  repository: string;
  branch: string;
  build_context: string;
  dockerfile_path: string;
  access_mode: "public" | "github_app";
  github_installation_id?: number;
  github_repository_id?: number;
  private: boolean;
  auto_deploy: boolean;
};

export type GitHubInstallation = {
  installation_id: number;
  account_login: string;
  account_type: string;
  repository_selection: string;
};

export type GitHubRepository = {
  id: number;
  full_name: string;
  private: boolean;
  default_branch: string;
};

export type EnvKey = {
  key: string;
  updated_at: string;
};
