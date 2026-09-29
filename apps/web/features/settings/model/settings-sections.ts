import type { Feature } from "@deeix/core";

// Which server capability a section depends on, if any. The sidebar and the
// route guard both read this table (docs/ARCHITECTURE.md §4).
export const SETTINGS_SECTIONS = [
  { id: "general", labelKey: "general", href: "/general" },
  { id: "chat", labelKey: "chat", href: "/chat" },
  { id: "credentials", labelKey: "credentials", href: "/credentials" },
  { id: "subscription", labelKey: "subscription", href: "/subscription", feature: "billingGating" },
  { id: "account", labelKey: "account", href: "/account" },
  { id: "about", labelKey: "about", href: "/about" },
] as const satisfies readonly { id: string; labelKey: string; href: string; feature?: Feature }[];

export type SettingsSection = (typeof SETTINGS_SECTIONS)[number]["id"];
export type SettingsSectionEntry = (typeof SETTINGS_SECTIONS)[number];

export const DEFAULT_SETTINGS_SECTION: SettingsSection = "general";

/** The section that owns `pathname` under `basePath`, or null for none. */
export function resolveSettingsSectionFromPath(pathname: string, basePath: string): SettingsSectionEntry | null {
  const normalizedBasePath = basePath.replace(/\/$/, "");
  return (
    SETTINGS_SECTIONS.find((entry) => {
      const href = `${normalizedBasePath}${entry.href}`;
      return pathname === href || pathname.startsWith(`${href}/`);
    }) ?? null
  );
}

export function resolveSettingsSection(section?: string | null): SettingsSection {
  if (SETTINGS_SECTIONS.some((item) => item.id === section)) {
    return section as SettingsSection;
  }
  return DEFAULT_SETTINGS_SECTION;
}
