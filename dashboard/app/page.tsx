"use client";

import { useRouter } from "next/navigation";
import { Landing } from "./components/Landing";
import { useTheme } from "./hooks/useTheme";

export default function Home() {
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();

  return (
    <Landing
      theme={theme}
      onThemeChange={toggleTheme}
      onAccess={() => router.push("/dashboard/projects")}
      apiIssue=""
    />
  );
}
