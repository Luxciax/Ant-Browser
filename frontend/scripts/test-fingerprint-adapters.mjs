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
} finally {
  rmSync(dir, { recursive: true, force: true })
}
