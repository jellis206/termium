import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import type { Browser, Cookie, CookieData, Page } from 'puppeteer';

// Persists cookies and localStorage across termium launches. Each launch runs
// Chromium on a throwaway profile, and several termiums may run at once, so the
// profile itself cannot be shared. Pages keep the native APIs: saved cookies are
// loaded into the browser, saved localStorage is restored before page scripts
// run, and changes (including deletions) are merged back into one JSON file.
//
// shortcut: read-merge-write without a file lock; two instances saving in the
// same millisecond can drop one side's latest change until its next save.

type LocalStore = Record<string, Record<string, string>>;
export type Store = { cookies: Record<string, CookieData>; local: LocalStore };

const SAVE_INTERVAL_MS = 2000;
const PAGE_READ_TIMEOUT_MS = 500;

// The client resolves TERMIUM_STORAGE_PROFILE from .termium-profile files; with
// no profile (and no explicit TERMIUM_STORAGE file) nothing persists.
export function storagePath(env = process.env): string | undefined {
    if (env.TERMIUM_STORAGE) return env.TERMIUM_STORAGE;
    const profile = env.TERMIUM_STORAGE_PROFILE;
    if (!profile || !/^[A-Za-z0-9._@+-]{1,128}$/.test(profile) || profile === '.' || profile === '..') return undefined;
    const data = env.XDG_DATA_HOME || path.join(os.homedir(), '.local', 'share');
    return path.join(data, 'termium', 'profiles', profile, 'storage.json');
}

export function readStore(file: string): Store {
    try {
        const parsed = JSON.parse(fs.readFileSync(file, 'utf8'));
        const cookies = parsed && typeof parsed.cookies === 'object' && parsed.cookies ? parsed.cookies : {};
        const local = parsed && typeof parsed.local === 'object' && parsed.local ? parsed.local : {};
        return { cookies, local };
    } catch {
        return { cookies: {}, local: {} };
    }
}

function writeStore(file: string, store: Store) {
    fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
    const temporary = `${file}.${process.pid}.tmp`;
    fs.writeFileSync(temporary, JSON.stringify(store), { mode: 0o600 });
    fs.renameSync(temporary, file);
}

// Keys this instance saw before and no longer has were deleted here; keys it
// never saw belong to another instance and are kept.
export function mergeEntries<V>(saved: Record<string, V>, previous: Iterable<string>, current: Record<string, V>): Record<string, V> {
    const merged = { ...saved };
    for (const key of previous) if (!(key in current)) delete merged[key];
    return Object.assign(merged, current);
}

// Runs before page scripts. Restores only into an empty store, so within a
// session the page's own localStorage stays authoritative.
export function hydrateScript(local: LocalStore): string {
    return `(() => { try {
        const data = (${JSON.stringify(local)})[location.origin];
        if (!data || localStorage.length) return;
        for (const [key, value] of Object.entries(data)) localStorage.setItem(key, value);
    } catch {} })();`;
}

const cookieKey = (c: { name: string; domain?: string; path?: string }) => `${c.name}\t${c.domain ?? ''}\t${c.path ?? '/'}`;

function cookieData(c: Cookie): CookieData {
    return { name: c.name, value: c.value, domain: c.domain, path: c.path, expires: c.expires, httpOnly: c.httpOnly, secure: c.secure, sameSite: c.sameSite };
}

function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T | undefined> {
    return Promise.race([promise, new Promise<undefined>(resolve => setTimeout(resolve, ms))]);
}

export class PersistentStorage {
    private seenCookies = new Set<string>();
    private seenLocal: Record<string, Set<string>> = {};
    private localJson = '{}';
    private scripts = new Map<Page, string>();
    private timer?: NodeJS.Timeout;
    private saving?: Promise<void>;

    constructor(private browser: Browser, private file: string, private log: (...args: unknown[]) => void = () => {}) {}

    async start() {
        const store = readStore(this.file);
        const now = Date.now() / 1000;
        const cookies = Object.values(store.cookies).filter(c => !c.expires || c.expires < 0 || c.expires > now);
        if (cookies.length) await this.browser.setCookie(...cookies);
        this.seenCookies = new Set(Object.keys(store.cookies));
        for (const [origin, items] of Object.entries(store.local)) this.seenLocal[origin] = new Set(Object.keys(items));
        this.localJson = JSON.stringify(store.local);
        this.browser.on('targetcreated', target => {
            if (target.type() === 'page') void target.page().then(page => page && this.attach(page)).catch(() => {});
        });
        const pages = await this.browser.pages();
        for (const page of pages) await this.attach(page).catch(() => {});
        this.timer = setInterval(() => void this.save().catch(error => this.log('save failed:', error)), SAVE_INTERVAL_MS);
        this.timer.unref();
        this.log('started with', cookies.length, 'cookies and', Object.keys(store.local).length, 'origins');
    }

    async stop() {
        if (this.timer) clearInterval(this.timer);
        await this.save().catch(error => this.log('save failed:', error));
    }

    private async attach(page: Page) {
        const previous = this.scripts.get(page);
        if (previous) await page.removeScriptToEvaluateOnNewDocument(previous).catch(() => {});
        const { identifier } = await page.evaluateOnNewDocument(hydrateScript(JSON.parse(this.localJson)));
        this.scripts.set(page, identifier);
        page.once('close', () => this.scripts.delete(page));
    }

    save(): Promise<void> {
        // One save at a time; an interval tick during shutdown joins it.
        this.saving ??= this.saveOnce().finally(() => { this.saving = undefined; });
        return this.saving;
    }

    private async saveOnce() {
        const store = readStore(this.file);
        const before = JSON.stringify(store);

        const current: Record<string, CookieData> = {};
        for (const cookie of await this.browser.cookies()) {
            if (!cookie.session) current[cookieKey(cookie)] = cookieData(cookie);
        }
        store.cookies = mergeEntries(store.cookies, this.seenCookies, current);
        this.seenCookies = new Set(Object.keys(current));

        const observed: LocalStore = {};
        for (const page of await this.browser.pages()) {
            if (!/^https?:/.test(page.url())) continue;
            const read = await withTimeout(page.evaluate(() => ({ origin: location.origin, items: { ...localStorage } })).catch(() => undefined), PAGE_READ_TIMEOUT_MS);
            if (read) observed[read.origin] = read.items as Record<string, string>;
        }
        for (const [origin, items] of Object.entries(observed)) {
            const merged = mergeEntries(store.local[origin] ?? {}, this.seenLocal[origin] ?? [], items);
            if (Object.keys(merged).length) store.local[origin] = merged;
            else delete store.local[origin];
            this.seenLocal[origin] = new Set(Object.keys(items));
        }

        if (JSON.stringify(store) !== before) writeStore(this.file, store);
        const localJson = JSON.stringify(store.local);
        if (localJson !== this.localJson) {
            this.localJson = localJson;
            for (const page of [...this.scripts.keys()]) await this.attach(page).catch(() => {});
        }
    }
}
