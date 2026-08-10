export type Theme = "light" | "dark";

export type App = {
  id: string;
  name: string;
  status: "idle" | "running" | "failed";
  container_state?: string;
  public_url?: string;
  created_at: string;
  updated_at: string;
};

export type Deployment = {
  id: string;
  image_tag: string;
  status: "pending" | "building" | "running" | "failed" | "superseded" | "rolled_back";
  port?: number;
  duration_ms?: number;
  created_at: string;
};

export type EnvKey = {
  key: string;
  updated_at: string;
};
