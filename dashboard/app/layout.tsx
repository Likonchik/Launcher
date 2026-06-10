import type { Metadata } from 'next';
import './globals.css';
import { AppFrame } from './ui/app-frame';

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
      <head>
        <link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@24,400,0,0" rel="stylesheet" />
      </head>
      <body>
        <AppFrame>{children}</AppFrame>
      </body>
    </html>
  );
}

