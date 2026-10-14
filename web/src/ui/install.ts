// "Install the app": shown when the browser offers to install BombayChitra (Chromium's beforeinstallprompt). Other
// browsers install from their own menus — Safari with Share → Add to Home Screen.
import { t } from '../i18n.ts';

interface InstallPrompt extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

export function setupInstall(button: HTMLButtonElement, toast: (message: string) => void): void {
  let offer: InstallPrompt | null = null;
  button.hidden = true;
  button.title = t('tool.install');
  button.setAttribute('aria-label', button.title);
  addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault(); // keep the offer for the button instead of the browser's mini bar
    offer = e as InstallPrompt;
    button.hidden = false;
  });
  addEventListener('appinstalled', () => {
    offer = null;
    button.hidden = true;
    toast(t('install.done'));
  });
  button.addEventListener('click', async () => {
    if (!offer) return;
    await offer.prompt();
    if ((await offer.userChoice).outcome === 'accepted') button.hidden = true;
    offer = null; // an offer can be shown once
  });
}
