import type { Metadata } from 'next';
import './globals.css';
import { AppFrame } from '../components/shell/app-frame';

export const metadata: Metadata = {
  title: 'Project Minecraft Admin',
  description: 'Administration dashboard for Project Minecraft'
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru">
      <body className="antialiased">
        <AppFrame>{children}</AppFrame>
      </body>
    </html>
  );
}

