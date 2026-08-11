"use client";

import { useEffect, useState } from "react";
import type { Theme } from "../types";

export function useTheme() {
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const saved = window.localStorage.getItem("minipaas-theme");
      if (saved === "light" || saved === "dark") setTheme(saved);
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    window.localStorage.setItem("minipaas-theme", theme);
  }, [theme]);

  return {
    theme,
    toggleTheme: () => setTheme((current) => current === "dark" ? "light" : "dark"),
  };
}
