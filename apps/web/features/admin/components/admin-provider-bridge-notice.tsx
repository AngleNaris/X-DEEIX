"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { getLoginOptions } from "@/shared/api/auth";
import { useCapabilities } from "@/shared/capabilities";

const DISMISSED_STORAGE_KEY = "deeix-chat:admin:provider-bridge-notice";

/**
 * Warns administrators once per session when identity providers are configured
 * but the server cannot run the OAuth handoff because PUBLIC_API_BASE_URL is
 * missing. Sign-in through those providers is unavailable until it is set.
 */
export function AdminProviderBridgeNotice({ basePath }: { basePath: string }) {
  const t = useTranslations("adminLogin.bridgeNotice");
  const router = useRouter();
  const { flags, loaded } = useCapabilities();
  const [open, setOpen] = React.useState(false);

  React.useEffect(() => {
    if (!loaded || !flags.identityProviders || window.sessionStorage.getItem(DISMISSED_STORAGE_KEY)) {
      return;
    }
    let cancelled = false;
    void getLoginOptions()
      .then((options) => {
        if (!cancelled && options.providers.length > 0 && !options.providerAuthBridge.enabled) {
          setOpen(true);
        }
      })
      .catch(() => {
        // The login settings page reports load failures itself.
      });
    return () => {
      cancelled = true;
    };
  }, [flags.identityProviders, loaded]);

  const dismiss = React.useCallback(() => {
    window.sessionStorage.setItem(DISMISSED_STORAGE_KEY, "1");
    setOpen(false);
  }, []);

  return (
    <AlertDialog open={open} onOpenChange={(next) => (next ? setOpen(true) : dismiss())}>
      <AlertDialogContent size="sm">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("title")}</AlertDialogTitle>
          <AlertDialogDescription>{t("description")}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={dismiss}>{t("dismiss")}</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              dismiss();
              router.push(`${basePath}/login`);
            }}
          >
            {t("openSettings")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
