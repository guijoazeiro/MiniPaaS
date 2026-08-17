import type { ReactNode } from "react";
import { DashboardShell } from "../components/DashboardShell";
import { GitHubOnboardingModal } from "../components/GitHubOnboardingModal";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return <DashboardShell><GitHubOnboardingModal />{children}</DashboardShell>;
}
