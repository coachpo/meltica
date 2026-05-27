'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { MenuIcon } from 'lucide-react';

import { ThemeToggle } from '@/components/theme-toggle';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';

const navItems = [
  { href: '/', label: 'Dashboard' },
  { href: '/instances', label: 'Instances' },
  { href: '/strategies/modules', label: 'Strategy Modules' },
  { href: '/providers', label: 'Providers' },
  { href: '/adapters', label: 'Adapters' },
  { href: '/risk', label: 'Risk Limits' },
  { href: '/context', label: 'Context Backup' },
];

export function Nav() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur">
      <div className="mx-auto flex w-full max-w-6xl items-center justify-between gap-4 px-4 py-3 md:px-6">
        <div className="flex items-center gap-3">
          <Link href="/" className="text-lg font-semibold tracking-tight text-foreground whitespace-nowrap">
            Meltica Control
          </Link>
          <Separator orientation="vertical" className="hidden h-5 md:block" />
          <DesktopNav pathname={pathname} />
        </div>
        <div className="flex items-center gap-2">
          <ThemeToggle />
          <MobileNav pathname={pathname} open={mobileOpen} onOpenChange={setMobileOpen} />
        </div>
      </div>
    </header>
  );
}

function DesktopNav({ pathname }: { pathname: string }) {
  return (
    <nav className="hidden items-center gap-2 md:flex">
      {navItems.map((item) => (
        <Button
          key={item.href}
          variant={pathname === item.href ? 'secondary' : 'navbar'}
          asChild
          className="px-3 py-1 text-sm font-medium"
        >
          <Link href={item.href}>{item.label}</Link>
        </Button>
      ))}
    </nav>
  );
}

type MobileNavProps = {
  pathname: string;
  open: boolean;
  onOpenChange(open: boolean): void;
};

function MobileNav({ pathname, open, onOpenChange }: MobileNavProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetTrigger asChild>
        <Button variant="outline" size="icon" className="md:hidden">
          <MenuIcon className="h-5 w-5" />
          <span className="sr-only">Open navigation</span>
        </Button>
      </SheetTrigger>
      <SheetContent side="left" className="flex flex-col px-0">
        <SheetHeader className="px-6 pt-4 text-left">
          <SheetTitle>Meltica Control</SheetTitle>
        </SheetHeader>
        <ScrollArea className="flex-1 px-2">
          <div className="flex flex-col gap-1 py-4">
            {navItems.map((item) => {
              const isActive = pathname === item.href;
              return (
                <Button
                  key={item.href}
                  variant={isActive ? 'secondary' : 'navbar'}
                  className="justify-start text-base"
                  asChild
                  onClick={() => onOpenChange(false)}
                >
                  <Link href={item.href}>{item.label}</Link>
                </Button>
              );
            })}
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  );
}
