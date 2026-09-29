import type { Feature } from "@deeix/core";

// Which server capability a section depends on, if any. The sidebar and the
// route guard both read this table, so a hidden entry is never reachable by URL
// either (docs/ARCHITECTURE.md §4).
export const ADMIN_SECTIONS = [
  { id: "statistics", label: "Statistics", href: "/statistics" },
  { id: "accounts", label: "Accounts", href: "/users", feature: "multiUser" },
  { id: "groups", label: "Permission Groups", href: "/groups", feature: "multiUser" },
  { id: "upstreams", label: "Upstreams", href: "/upstreams" },
  { id: "models", label: "Models", href: "/models" },
  { id: "tool-settings", label: "Tools", href: "/tools" },
  { id: "billing", label: "Billing", href: "/billing" },
  { id: "announcements", label: "Announcements", href: "/announcements", feature: "announcements" },
  { id: "logs", label: "Logs", href: "/logs" },
  { id: "content-moderation", label: "Content moderation", href: "/content-moderation", feature: "contentModeration" },
  { id: "login-settings", label: "Login & auth", href: "/login", feature: "identityProviders" },
  { id: "conversation-settings", label: "Conversation", href: "/conversation" },
  { id: "agent-groups", label: "Agent Groups", href: "/agent-groups" },
  { id: "platform-tools", label: "Platform Tools", href: "/platform-tools" },
  { id: "chat-files", label: "Files & retrieval", href: "/chat-files" },
  { id: "knowledge-bases", label: "Knowledge bases", href: "/knowledge-bases" },
  { id: "about", label: "About", href: "/about" },
] as const satisfies readonly { id: string; label: string; href: string; feature?: Feature }[];

export type AdminSection = (typeof ADMIN_SECTIONS)[number]["id"];
export type AdminSectionEntry = (typeof ADMIN_SECTIONS)[number];

export const DEFAULT_ADMIN_SECTION: AdminSection = "statistics";

/** The section that owns `pathname` under `basePath`, or null for none. */
export function resolveAdminSectionFromPath(pathname: string, basePath: string): AdminSectionEntry | null {
  const normalizedBasePath = basePath.replace(/\/$/, "");
  return (
    ADMIN_SECTIONS.find((entry) => {
      const href = `${normalizedBasePath}${entry.href}`;
      return pathname === href || pathname.startsWith(`${href}/`);
    }) ?? null
  );
}

export function resolveAdminSection(section?: string | null): AdminSection {
  if (ADMIN_SECTIONS.some((item) => item.id === section)) {
    return section as AdminSection;
  }
  return DEFAULT_ADMIN_SECTION;
}
