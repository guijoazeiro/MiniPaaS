export type Theme = "light" | "dark";

export type App = {
  id: string;
  name: string;
  status: "idle" | "running" | "failed" | "stopped";
  container_state?: string;
  public_url?: string;
  created_at: string;
  updated_at: string;
};

export type Deployment = {
  id: string;
  app_id: string;
  app_name?: string;
  image_tag: string;
  status: "pending" | "building" | "running" | "failed" | "superseded" | "rolled_back" | "stopped";
  port?: number;
  duration_ms?: number;
  created_at: string;
  source_type?: "upload" | "git";
  repository?: string;
  branch?: string;
  commit_sha?: string;
  commit_author?: string;
  commit_message?: string;
};

export type DeploymentPage = {
  items: Deployment[];
  page: number;
  per_page: number;
  total: number;
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
