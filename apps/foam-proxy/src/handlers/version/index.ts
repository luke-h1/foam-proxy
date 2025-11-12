import * as Sentry from '@sentry/serverless';

const versionHandler = () => {
  return Sentry.startSpan(
    {
      name: 'versionHandler',
      op: 'function.version',
    },
    () => {
      return JSON.stringify(
        {
          deployedBy: process.env.DEPLOYED_BY ?? 'unknown',
          deployedAt: process.env.DEPLOYED_AT ?? 'unknown',
          gitSha: process.env.GIT_SHA ?? 'unknown',
        },
        null,
        2,
      );
    },
  );
};
export default versionHandler;
