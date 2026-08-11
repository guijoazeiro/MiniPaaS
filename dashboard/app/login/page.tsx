"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { LoginScreen } from "../components/LoginScreen";
import { ApiError, request } from "../lib/api";
import { useTheme } from "../hooks/useTheme";

export default function LoginPage() {
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [apiIssue, setApiIssue] = useState("");

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setApiIssue("");
    const form = new FormData(event.currentTarget);
    try {
      await request("/auth/web-login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: form.get("username"), password: form.get("password") }),
      });
      router.replace("/dashboard/projects");
    } catch (cause) {
      if (cause instanceof ApiError && cause.status !== 401) setApiIssue(cause.message);
      setError(cause instanceof Error ? cause.message : "Não foi possível entrar.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <LoginScreen
      theme={theme}
      onThemeChange={toggleTheme}
      onBack={() => router.push("/")}
      onLogin={login}
      error={error}
      apiIssue={apiIssue}
      busy={busy}
    />
  );
}
