'use strict';

// トークンの値は出さず、ファイルは判定の成否によらず削除する
const fs = require('node:fs');
const https = require('node:https');
const path = require('node:path');

const dir = path.join(process.env.RUNNER_TEMP, '_e2e-revoked-token');
const file = path.join(dir, 'token');

function fail(message) {
  console.log(`::error::${message}`);
  process.exitCode = 1;
}

// 宛先のホストは固定する。repository はパスに入るため、呼び出し元がオーナー名とリポジトリ名の文字に限ってから渡す。
// 応答が本文の途中で切れた場合も、待ち続けずに失敗として返す
function readme(repository, token) {
  return new Promise((resolve, reject) => {
    const request = https.request(
      {
        host: 'api.github.com',
        path: `/repos/${repository}/contents/README.md`,
        method: 'GET',
        headers: {
          Accept: 'application/vnd.github+json',
          Authorization: `Bearer ${token}`,
          'User-Agent': 'gates-e2e-revoked-token',
          'X-GitHub-Api-Version': '2022-11-28',
        },
        timeout: 10000,
      },
      (response) => {
        response.resume();
        response.on('end', () => resolve(response.statusCode));
        response.on('error', reject);
        response.on('close', () => {
          if (!response.complete) {
            reject(new Error('the response was cut off'));
          }
        });
      },
    );
    request.on('timeout', () => request.destroy(new Error('request timed out')));
    request.on('error', reject);
    request.end();
  });
}

async function check() {
  const repository = process.env.INPUT_REPOSITORY || '';
  if (!/^[A-Za-z0-9-]+\/[A-Za-z0-9._-]+$/.test(repository)) {
    fail('input repository is not an owner/repository full name');
    return;
  }
  let token;
  try {
    token = fs.readFileSync(file, 'utf8');
  } catch {
    fail('the token file is missing; the later step did not write the token');
    return;
  }
  if (token === '') {
    fail('the token file is empty');
    return;
  }

  let status;
  try {
    status = await readme(repository, token);
  } catch (error) {
    fail(`the request with the revoked token failed: ${error.message}`);
    return;
  }
  console.log(`revoked token read README.md: HTTP status ${status}`);
  if (status === 200) {
    fail('the token is still usable; the post-entrypoint did not revoke it before this post');
  } else if (status !== 401) {
    fail('the revoked token was not rejected with 401');
  }
}

check()
  .catch((error) => fail(`unexpected error: ${error.message}`))
  .finally(() => {
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch (error) {
      fail(`could not remove the token file: ${error.code}`);
    }
    if (fs.existsSync(dir)) {
      fail('the token file was not removed');
    } else {
      console.log('token file removed');
    }
  });
