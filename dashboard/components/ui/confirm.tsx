'use client';

import { createContext, useContext, useRef, useState } from 'react';
import { Modal } from './modal';
import { Button } from './button';

type ConfirmOptions = {
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
};

const ConfirmContext = createContext<(opts: ConfirmOptions) => Promise<boolean>>(async () => false);

export function useConfirm() {
  return useContext(ConfirmContext);
}

export function ConfirmProvider({ children }: { children: React.ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolver = useRef<((ok: boolean) => void) | null>(null);

  const confirm = (options: ConfirmOptions) =>
    new Promise<boolean>((resolve) => {
      resolver.current = resolve;
      setOpts(options);
    });

  const finish = (ok: boolean) => {
    resolver.current?.(ok);
    resolver.current = null;
    setOpts(null);
  };

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <Modal
        open={opts !== null}
        onClose={() => finish(false)}
        title={opts?.title ?? ''}
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => finish(false)}>
              Отмена
            </Button>
            <Button variant={opts?.danger ? 'danger' : 'primary'} onClick={() => finish(true)}>
              {opts?.confirmLabel ?? 'Подтвердить'}
            </Button>
          </div>
        }
      >
        <p className="text-sm text-fg-secondary">{opts?.message}</p>
      </Modal>
    </ConfirmContext.Provider>
  );
}
