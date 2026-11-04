const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const { mergeEntries, readStore, hydrateScript, storagePath } = require('../dist/src/persistent-storage');

test('merge keeps other instances keys and drops keys deleted here', () => {
    const saved = { a: 1, b: 2, other: 9 };
    assert.deepEqual(mergeEntries(saved, ['a', 'b'], { b: 3, c: 4 }), { b: 3, c: 4, other: 9 });
});

test('a missing or corrupt store reads as empty', () => {
    const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'termium-store-')), 'storage.json');
    assert.deepEqual(readStore(file), { cookies: {}, local: {} });
    fs.writeFileSync(file, '{not json');
    assert.deepEqual(readStore(file), { cookies: {}, local: {} });
    fs.rmSync(path.dirname(file), { recursive: true });
});

test('hydration fills only an empty localStorage for its origin', () => {
    const run = (origin, existing) => {
        const items = new Map(Object.entries(existing));
        const localStorage = { get length() { return items.size; }, setItem: (k, v) => items.set(k, v) };
        vm.runInNewContext(hydrateScript({ 'http://a': { k: 'v' } }), { location: { origin }, localStorage });
        return Object.fromEntries(items);
    };
    assert.deepEqual(run('http://a', {}), { k: 'v' });
    assert.deepEqual(run('http://a', { mine: 'x' }), { mine: 'x' });
    assert.deepEqual(run('http://b', {}), {});
});

test('storage path needs a valid profile unless a file is given', () => {
    assert.equal(storagePath({}), undefined);
    assert.equal(storagePath({ TERMIUM_STORAGE_PROFILE: '..' }), undefined);
    assert.equal(storagePath({ TERMIUM_STORAGE_PROFILE: 'a/b' }), undefined);
    assert.equal(storagePath({ TERMIUM_STORAGE: '/x.json' }), '/x.json');
    assert.equal(storagePath({ TERMIUM_STORAGE_PROFILE: 'jay@enzo.health', XDG_DATA_HOME: '/d' }), '/d/termium/profiles/jay@enzo.health/storage.json');
});
