'use client';

import { useCallback, type ReactNode } from 'react';
import { toast } from 'sonner';

import { Toaster } from '@/components/ui/sonner';

type ToastVariant = 'default' | 'destructive' | 'success' | 'info' | 'warning';

interface ToastOptions {
  title?: string;
  description?: string;
  variant?: ToastVariant;
  duration?: number;
}

type ToastId = string | number;

function resolveHandler(variant: ToastVariant | undefined) {
  switch (variant) {
    case 'destructive':
      return toast.error;
    case 'success':
      return toast.success;
    case 'info':
      return toast.info;
    case 'warning':
      return toast.warning;
    default:
      return toast;
  }
}

export function ToastProvider({ children }: { children: ReactNode }) {
  return (
    <>
      {children}
      <Toaster position="top-right" expand closeButton richColors />
    </>
  );
}

export function useToast() {
  const show = useCallback((options: ToastOptions) => {
    const handler = resolveHandler(options.variant);
    const hasTitle = Boolean(options.title);
    const message = hasTitle ? options.title! : options.description ?? '';

    return handler(message, {
      description: hasTitle ? options.description : undefined,
      duration: options.duration,
    });
  }, []);

  const dismiss = useCallback((id?: ToastId) => {
    toast.dismiss(id);
  }, []);

  return { show, dismiss };
}
