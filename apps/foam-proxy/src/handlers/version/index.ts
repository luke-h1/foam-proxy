import * as Sentry from '@sentry/aws-serverless';

const versionHandler = () => {
  Sentry.captureMessage('versionHandler');
  return JSON.stringify(
    {
      deployedBy: process.env.DEPLOYED_BY ?? 'unknown',
      deployedAt: process.env.DEPLOYED_AT ?? 'unknown',
      gitSha: process.env.GIT_SHA ?? 'unknown',
    },
    null,
    2,
  );
};
export default versionHandler;
