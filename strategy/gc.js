#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const strategiesRoot = path.resolve(__dirname);
const registryFile = path.join(strategiesRoot, 'registry.json');

if (!fs.existsSync(registryFile)) {
  console.error(`Cannot find registry file at ${registryFile}`);
  process.exit(1);
}

let registry;
try {
  registry = JSON.parse(fs.readFileSync(registryFile, 'utf8'));
} catch (err) {
  console.error('Failed to parse registry.json:', err.message);
  process.exit(1);
}

const allowedJsFiles = new Set();
const registeredDirectories = new Set(['.']);
let registryHasEntries = false;

const addRegisteredDirectories = (filePath) => {
  let currentDir = path.dirname(filePath);
  while (
    currentDir &&
    (currentDir === strategiesRoot || currentDir.startsWith(`${strategiesRoot}${path.sep}`))
  ) {
    const relativeDir = path.relative(strategiesRoot, currentDir) || '.';
    registeredDirectories.add(relativeDir);
    if (currentDir === strategiesRoot) {
      break;
    }
    currentDir = path.dirname(currentDir);
  }
};

const hexDigest = /^[0-9a-f]{64}$/i;

Object.entries(registry || {}).forEach(([strategyName, strategy]) => {
  if (!strategy || typeof strategy !== 'object') return;
  const hashes = strategy.hashes || {};
  Object.entries(hashes).forEach(([hashKey, hashInfo]) => {
    if (!hashInfo || !hashInfo.path) return;
    const digest = (hashKey || '').replace(/^sha256:/i, '');
    if (!hexDigest.test(digest)) {
      console.error(
        `Hash ${hashKey} for ${strategyName} must be a 64-character hex digest.`
      );
      process.exit(1);
    }
    const expectedRelative = path.join(strategyName, digest, `${strategyName}.js`);
    const normalizedRelative = path.normalize(hashInfo.path);
    if (normalizedRelative !== expectedRelative) {
      console.error(
        `Registry path mismatch for ${strategyName}@${hashKey}: expected ${expectedRelative}, found ${hashInfo.path}`
      );
      process.exit(1);
    }
    const normalized = path.resolve(strategiesRoot, normalizedRelative);
    if (
      normalized === strategiesRoot ||
      !normalized.startsWith(`${strategiesRoot}${path.sep}`)
    ) {
      return;
    }
    allowedJsFiles.add(normalized);
    addRegisteredDirectories(normalized);
    registryHasEntries = true;
  });
});

const protectedFiles = [path.resolve(__filename)];
protectedFiles.forEach((filePath) => {
  allowedJsFiles.add(filePath);
  addRegisteredDirectories(filePath);
});

if (!registryHasEntries) {
  console.warn('No registered JS files detected. Exiting without changes.');
  process.exit(0);
}

const skipDirectories = new Set(['.git', '.github', 'node_modules']);
const removedFiles = [];
const removedDirs = [];

const removeEmptyDirectories = (dir) => {
  if (dir === strategiesRoot) return false;
  if (!fs.existsSync(dir)) return false;
  const relativeDirRaw = path.relative(strategiesRoot, dir);
  if (relativeDirRaw.startsWith('..')) {
    return false;
  }
  const relativeDir = relativeDirRaw || '.';
  if (registeredDirectories.has(relativeDir)) {
    return false;
  }
  const entries = fs.readdirSync(dir);
  if (entries.length === 0) {
    fs.rmdirSync(dir);
    removedDirs.push(relativeDir);
    return true;
  }
  return false;
};

const pruneDirectory = (dir) => {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (skipDirectories.has(entry.name)) {
      continue;
    }
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      pruneDirectory(fullPath);
      if (!fs.existsSync(fullPath)) {
        continue;
      }
      removeEmptyDirectories(fullPath);
    } else if (entry.isFile()) {
      if (entry.name.endsWith('.js')) {
        const resolved = path.resolve(fullPath);
        if (!allowedJsFiles.has(resolved)) {
          fs.unlinkSync(fullPath);
          removedFiles.push(path.relative(strategiesRoot, fullPath));
        }
      }
    }
  }

  if (dir !== strategiesRoot) {
    removeEmptyDirectories(dir);
  }
};

pruneDirectory(strategiesRoot);

if (removedFiles.length === 0 && removedDirs.length === 0) {
  console.log('GC complete. No cleanup required.');
} else {
  if (removedFiles.length) {
    console.log('Removed JS files:');
    removedFiles.forEach((file) => console.log(` - ${file}`));
  }
  if (removedDirs.length) {
    console.log('Removed empty directories:');
    removedDirs.forEach((dir) => console.log(` - ${dir || '.'}`));
  }
}
