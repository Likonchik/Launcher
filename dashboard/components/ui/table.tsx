import { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from 'react';

export function Table({ className = '', children }: { className?: string; children: React.ReactNode }) {
  return (
    <div className={`overflow-x-auto rounded-xl border border-edge bg-surface backdrop-blur-xl ${className}`}>
      <table className="w-full text-sm">{children}</table>
    </div>
  );
}

export function Th({ className = '', children, ...rest }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th {...rest} className={`px-3 py-2.5 text-left text-xs font-medium uppercase tracking-wide text-fg-faint ${className}`}>
      {children}
    </th>
  );
}

export function Td({ className = '', children, ...rest }: TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td {...rest} className={`border-t border-edge/60 px-3 py-2.5 ${className}`}>
      {children}
    </td>
  );
}

export function ClickableRow({ className = '', children, ...rest }: HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr {...rest} className={`cursor-pointer transition hover:bg-surface ${className}`}>
      {children}
    </tr>
  );
}
