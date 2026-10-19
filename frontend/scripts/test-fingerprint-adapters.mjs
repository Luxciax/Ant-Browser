import assert from 'node:assert/strict'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { buildSync } from 'esbuild'

const dir = mkdtempSync(join(tmpdir(), 'ant-fingerprint-tests-'))
try {
  const output = join(dir, 'adapters.mjs')
  buildSync({ entryPoints: ['src/modules/browser/utils/fingerprintSerializer.ts'], bundle: true, format: 'esm', platform: 'node', outfile: output })
  const legacy = await import(pathToFileURL(output).href)
  const input = { seed: '12345', platform: 'mac', brand: 'Chrome', brandVersion: '144.0.0.1', hardwareConcurrency: '8', deviceMemory: '8', colorDepth: '24', touchPoints: '7', webglVendor: 'legacy', lang: 'en-US', timezone: 'UTC', resolution: '1920,1080', canvasNoise: '1', clientRectsNoise: '0', disableSpoofing: ['GPU', 'font', 'gpu'], unknownArgs: ['--custom=value'] }
  const expected = ['--fingerprint=12345', '--fingerprint-brand=Chrome', '--fingerprint-brand-version=144.0.0.1', '--fingerprint-platform=macos', '--lang=en-US', '--accept-lang=en-US,en', '--timezone=UTC', '--window-size=1920,1080', '--fingerprint-hardware-concurrency=8', '--fingerprinting-canvas-image-data-noise', '--fingerprinting-client-rects-noise=0', '--disable-spoofing=gpu,font', '--custom=value']
  assert.deepEqual(legacy.serialize(input), expected, 'legacy output and ordering must remain unchanged')
  assert.deepEqual(legacy.serialize(legacy.deserialize(expected)), expected, 'legacy round trip')
  assert.equal(legacy.validateFingerprintArgs(['--fingerprint=not-numeric']).valid, false)
  console.log('fingerprint-chromium regression passed')
  const adapterOutput = join(dir, 'registry.mjs')
  buildSync({ entryPoints: ['src/modules/browser/utils/fingerprintAdapters/index.ts'], bundle: true, format: 'esm', platform: 'node', outfile: adapterOutput })
  const { getFingerprintAdapter, storeFingerprintConfig } = await import(pathToFileURL(adapterOutput).href)
  const clearcote = getFingerprintAdapter('clearcote')
  const config = { seed: '12345', platform: 'windows', hardwareConcurrency: '8', deviceMemory: '8', screenWidth: '1920', touchPoints: '0', devicePixelRatio: '1.25', mode: 'manual' }
  const args = clearcote.serialize(config)
  for (const arg of ['--fingerprint=12345', '--fingerprint-hardware-concurrency=8', '--fingerprint-device-memory=8', '--fingerprint-screen-width=1920', '--fingerprint-max-touch-points=0', '--fingerprint-device-pixel-ratio=1.25']) assert.ok(args.includes(arg), arg)
  const auto = clearcote.serialize({ ...config, mode: 'auto' })
  assert.ok(!auto.some(arg => /hardware-concurrency|device-memory|screen-width|max-touch-points|device-pixel-ratio/.test(arg)))
  assert.deepEqual(clearcote.deserialize(storeFingerprintConfig(clearcote, { ...config, mode: 'auto' })).deviceMemory, '8', 'dormant manual settings survive automatic mode')
  assert.ok(!getFingerprintAdapter('generic-chromium').serialize(config).some(arg => arg.startsWith('--fingerprint')))
  assert.ok(!getFingerprintAdapter('fingerprint-chromium').serialize(config).includes('--fingerprint-device-memory=8'))
  assert.equal(clearcote.validate({ ...config, devicePixelRatio: '-1' }).valid, false)
  assert.equal(clearcote.validate({ ...config, seed: 'string-seed' }).valid, true)
  const generic = getFingerprintAdapter('generic-chromium')
  const genericEdited = generic.deserialize(storeFingerprintConfig(clearcote, config))
  const returned = clearcote.deserialize(storeFingerprintConfig(generic, { ...genericEdited, lang: 'en-US' }))
  assert.equal(returned.deviceMemory, '8', 'Clearcote configuration must survive editing a generic kernel')
  assert.ok(clearcote.serialize(returned).includes('--fingerprint-device-memory=8'))
  assert.equal(clearcote.deserialize(['--fingerprint-webgl-vendor=legacy']).webglVendor, 'legacy')
  console.log('Clearcote mappings, mode isolation, validation and persistence passed')
  const downloadOutput = join(dir, 'downloads.mjs')
  buildSync({ entryPoints: ['src/modules/browser/pages/coreManagement/coreDownloadRecommendation.ts'], bundle: true, format: 'esm', platform: 'node', outfile: downloadOutput })
  const { fetchCoreDownloadRecommendation } = await import(pathToFileURL(downloadOutput).href)
  const originalFetch = globalThis.fetch
  try {
    let requestedURL = ''
    globalThis.fetch = async url => {
      requestedURL = String(url)
      return { ok: true, json: async () => ({ tag_name: 'v0.1.0-pre.23', assets: [{ name: 'clearcote-150-windows-x64.zip', browser_download_url: 'https://github.com/clearcotelabs/clearcote-browser/releases/download/test/clearcote-150-windows-x64.zip' }] }) }
    }
    const recommendation = await fetchCoreDownloadRecommendation({ platform: 'windows', arch: 'amd64' }, undefined, 'clearcote')
    assert.equal(requestedURL, 'https://api.github.com/repos/clearcotelabs/clearcote-browser/releases/latest')
    assert.equal(recommendation?.assetName, 'clearcote-150-windows-x64.zip')
    assert.equal(recommendation?.releasesUrl, 'https://github.com/clearcotelabs/clearcote-browser/releases')
    console.log('Clearcote Release recommendation passed')
  } finally { globalThis.fetch = originalFetch }
} finally {
  rmSync(dir, { recursive: true, force: true })
}
