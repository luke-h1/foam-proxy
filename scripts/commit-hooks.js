/* eslint-disable */
// @ts-nocheck
const { exec } = require('child_process');
const { promisify } = require('util');
const execAsync = promisify(exec);

async function main() {
  try {
    void execAsync('node ./scripts/check-semver.js');
  } catch (error) {
    console.error(error);
    process.exit(1);
  }
}

main();
