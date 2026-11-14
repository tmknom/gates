'use strict';

// ディレクトリが既にあれば、残っていたファイルで試験が通らないよう失敗させる
const fs = require('node:fs');
const path = require('node:path');

const dir = path.join(process.env.RUNNER_TEMP, '_e2e-revoked-token');
fs.mkdirSync(dir, { mode: 0o700 });
const file = path.join(dir, 'token');
fs.appendFileSync(process.env.GITHUB_OUTPUT, `token-file=${file}\n`);
console.log(`token file: ${file}`);
