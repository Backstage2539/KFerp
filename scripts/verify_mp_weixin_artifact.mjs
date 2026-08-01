#!/usr/bin/env node

import { readFile, stat, writeFile } from 'node:fs/promises'
import path from 'node:path'

const artifactRoot = process.argv[2]
const manifestPath = process.argv[3]
if (!artifactRoot) {
  console.error('Usage: verify_mp_weixin_artifact.mjs <mp-weixin-directory> [manifest-file]')
  process.exit(2)
}

const absoluteRoot = path.resolve(artifactRoot)
let appConfig
try {
  appConfig = JSON.parse(await readFile(path.join(absoluteRoot, 'app.json'), 'utf8'))
} catch (error) {
  console.error(`ERROR: cannot read mp-weixin app.json: ${error.message}`)
  process.exit(1)
}

const declaredPages = []

function addPage(pageEntry, packageRoot = '') {
  const pagePath = typeof pageEntry === 'string' ? pageEntry : pageEntry?.path
  if (typeof pagePath !== 'string' || !pagePath.trim()) {
    throw new Error('app.json contains an invalid page entry')
  }
  const combined = packageRoot ? `${packageRoot}/${pagePath}` : pagePath
  if (
    path.posix.isAbsolute(combined) ||
    combined.split('/').includes('..') ||
    combined.includes('\\') ||
    /[\r\n]/.test(combined)
  ) {
    throw new Error(`app.json contains an unsafe page path: ${combined}`)
  }
  declaredPages.push(path.posix.normalize(combined))
}

try {
  for (const page of appConfig.pages ?? []) {
    addPage(page)
  }
  for (const subPackage of appConfig.subPackages ?? appConfig.subpackages ?? []) {
    const packageRoot = subPackage?.root
    if (typeof packageRoot !== 'string' || !packageRoot.trim()) {
      throw new Error('app.json contains a subpackage without a root')
    }
    for (const page of subPackage.pages ?? []) {
      addPage(page, packageRoot)
    }
  }
} catch (error) {
  console.error(`ERROR: cannot enumerate mp-weixin pages: ${error.message}`)
  process.exit(1)
}

if (declaredPages.length === 0) {
  console.error('ERROR: mp-weixin app.json does not declare any pages')
  process.exit(1)
}

const requiredFiles = []
const missing = []
for (const page of declaredPages) {
  for (const extension of ['.js', '.json', '.wxml', '.wxss']) {
    const relativeFile = `${page}${extension}`
    requiredFiles.push(relativeFile)
    try {
      const fileInfo = await stat(path.join(absoluteRoot, ...relativeFile.split('/')))
      if (!fileInfo.isFile()) {
        missing.push(relativeFile)
      }
    } catch {
      missing.push(relativeFile)
    }
  }
}

if (missing.length > 0) {
  for (const relativeFile of missing) {
    console.error(`ERROR: mp-weixin artifact is missing declared page file: ${relativeFile}`)
  }
  process.exit(1)
}

if (manifestPath) {
  await writeFile(path.resolve(manifestPath), `${requiredFiles.join('\n')}\n`, 'utf8')
}

console.log(`Verified ${declaredPages.length} declared mp-weixin pages.`)
