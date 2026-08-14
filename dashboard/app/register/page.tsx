"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError, request } from "../lib/api";
import { RegisterScreen } from "../components/RegisterScreen";
import { useTheme } from "../hooks/useTheme";

export default function RegisterPage() {
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [apiIssue, setApiIssue] = useState("");

  async function register(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setApiIssue("");
    const form = new FormData(event.currentTarget);
    if (form.get("password") !== form.get("password_confirmation")) {
      setError("As senhas não coincidem.");
      setBusy(false);
      return;
    }
    try {
      await request("/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: form.get("username"), password: form.get("password") }),
      });
      router.replace("/login?registered=1");
    } catch (cause) {
      if (cause instanceof ApiError && cause.status >= 500) setApiIssue(cause.message);
      setError(cause instanceof Error ? cause.message : "Não foi possível criar a conta.");
    } finally {
      setBusy(false);
    }
  }

  return <RegisterScreen theme={theme} onThemeChange={toggleTheme} onBack={() => router.push("/")} onLogin={() => router.push("/login")} onRegister={register} error={error} apiIssue={apiIssue} busy={busy} />;
}
