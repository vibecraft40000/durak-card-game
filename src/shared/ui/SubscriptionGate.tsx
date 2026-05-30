import { useLanguage } from "@/shared/providers/LanguageProvider";

const DEFAULT_CHANNEL_LINK = "https://t.me/+P_rbSS0y5N9jM2Qy";

type Props = {
  channelLink?: string;
  onRetry: () => void;
  isRetrying?: boolean;
};

export function SubscriptionGate({ channelLink, onRetry, isRetrying }: Props) {
  const { t } = useLanguage();
  const link = channelLink || DEFAULT_CHANNEL_LINK;

  return (
    <div
      style={{
        minHeight: "100dvh",
        background: "var(--color-bg-primary, #010a1b)",
        color: "var(--color-text-primary, #f5f5f7)",
        padding: "24px 20px",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        fontFamily: "system-ui, sans-serif",
        textAlign: "center",
        boxSizing: "border-box",
      }}
    >
      <h1
        style={{
          fontSize: "1.5rem",
          marginBottom: "16px",
          fontWeight: 600,
        }}
      >
        {t("subscription.title")}
      </h1>
      <p
        style={{
          color: "var(--color-text-secondary, #8d8d93)",
          marginBottom: "20px",
          lineHeight: 1.5,
          maxWidth: "360px",
        }}
      >
        {t("subscription.description")}
      </p>
      <a
        href={link}
        target="_blank"
        rel="noopener noreferrer"
        style={{
          display: "inline-block",
          marginBottom: "16px",
          padding: "12px 24px",
          background: "var(--color-accent, #0a84ff)",
          color: "#fff",
          borderRadius: "12px",
          textDecoration: "none",
          fontWeight: 600,
        }}
      >
        {t("subscription.subscribe")}
      </a>
      <p
        style={{
          fontSize: "0.9rem",
          color: "var(--color-text-secondary, #8d8d93)",
          marginBottom: "16px",
        }}
      >
        {t("subscription.afterSubscribe")}
      </p>
      <button
        type="button"
        onClick={onRetry}
        disabled={isRetrying}
        style={{
          padding: "12px 24px",
          background: isRetrying ? "var(--color-bg-tertiary, #2c2c2e)" : "var(--color-bg-secondary, #1c1c1e)",
          color: "var(--color-text-primary, #f5f5f7)",
          border: "none",
          borderRadius: "12px",
          fontWeight: 600,
          cursor: isRetrying ? "wait" : "pointer",
        }}
      >
        {isRetrying ? t("app.loading") : t("subscription.check")}
      </button>
    </div>
  );
}
