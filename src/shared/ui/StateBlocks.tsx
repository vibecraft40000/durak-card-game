import type { ReactNode } from "react";
import { useLanguage } from "@/shared/providers/LanguageProvider";

type StateBlockProps = {
  title: string;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
};

export function EmptyStateBlock({ title, message, actionLabel, onAction }: StateBlockProps) {
  return (
    <div className="state-block">
      <div className="state-block__title">{title}</div>
      <div className="state-block__message">{message}</div>
      {actionLabel && onAction && (
        <button className="button button--primary" type="button" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  );
}

export function ErrorStateBlock({ title, message, actionLabel, onAction }: StateBlockProps) {
  return (
    <div className="state-block state-block--error">
      <div className="state-block__title">{title}</div>
      <div className="state-block__message">{message}</div>
      {actionLabel && onAction && (
        <button className="button" type="button" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  );
}

type SkeletonProps = {
  rows?: number;
};

export function CardSkeleton({ rows = 3 }: SkeletonProps) {
  return (
    <div className="card skeleton">
      {Array.from({ length: rows }).map((_, index) => (
        <div className="skeleton__line" key={`skeleton-${index}`} />
      ))}
    </div>
  );
}

type ModalProps = {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  cancelLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  footerExtra?: ReactNode;
};

export function ConfirmModal({
  isOpen,
  title,
  message,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onCancel,
  footerExtra,
}: ModalProps) {
  const { language } = useLanguage();
  const defaultCancelLabel = language === "uk" ? "Скасувати" : "Отмена";

  if (!isOpen) {
    return null;
  }

  return (
    <div className="modal-backdrop" role="presentation" onClick={onCancel}>
      <div className="modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
        <div className="modal__title">{title}</div>
        <div className="modal__message">{message}</div>
        {footerExtra}
        <div className="modal__actions">
          <button className="button" type="button" onClick={onCancel}>
            {cancelLabel ?? defaultCancelLabel}
          </button>
          <button className="button button--primary" type="button" onClick={onConfirm}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
